// Package specexamples extracts the textual-notation examples of the OMG SysML
// v2 and KerML specifications from the `pdftotext -layout` rendering of the
// pinned PDFs into one fixture per example, so that examples/spec-examples can
// be gated (tests/corpus) and refereed against the pilot (tools/referee/diff).
//
// Everything here reads the layout text alone; the rules are in Extract and are
// reproduced in docs/project/spec-examples.md. The published text is never
// edited: a fixture is the example's lines, dedented, plus a declared
// completion (Complete) that the file header states.
package specexamples

import (
	"regexp"
	"strconv"
	"strings"
)

// Document names one specification PDF rendered with `pdftotext -layout`.
type Document struct {
	// Name labels the output subdirectory and the fixture header: "sysml" or "kerml".
	Name string
	// Ext is the notation the document's examples are written in unless an
	// example says otherwise: "sysml" or "kerml".
	Ext string
	// Clauses are the clause-number prefixes whose examples are extracted; "7"
	// covers every 7.x subclause.
	Clauses []string
}

// Documents are the two specifications the pin names, in extraction order.
var Documents = []Document{
	{Name: "sysml", Ext: "sysml", Clauses: []string{"7", "8.4"}},
	{Name: "kerml", Ext: "kerml", Clauses: []string{"7"}},
}

// Block is one example as the text shows it.
type Block struct {
	Document string
	Clause   string
	// Page is the page number the running header prints for the page the
	// example starts on.
	Page string
	// Line is the 1-based line in the layout text where the example starts.
	Line int
	// Lines are the example's lines with the common indentation removed.
	Lines []string
	// Ext is the notation the example is written in: "sysml" or "kerml".
	Ext string
	// Index is the example's 1-based ordinal within its clause, among the
	// extracted examples.
	Index int
}

// Excluded is an indented, terminated block in an extracted clause that is not
// a usable example, with the rule that excluded it.
type Excluded struct {
	Clause string
	Page   string
	Line   int
	Reason string
	First  string
}

// Result is what Extract found in one document.
type Result struct {
	// Title is the specification's name as its running header prints it.
	Title    string
	Blocks   []Block
	Excluded []Excluded
}

var (
	// pageHeader is the running header pdftotext prints once per page, with the
	// page number on either side of the title.
	pageHeader = regexp.MustCompile(`^\s*(\d+|[ivxlc]+)?\s*((?:Systems|Kernel) Modeling Language v\d+\.\d+, Beta \d+(?:, Part \d+)?)\s*(\d+|[ivxlc]+)?\s*$`)
	// heading is a numbered clause heading at the left margin.
	heading = regexp.MustCompile(`^(\d+(?:\.\d+)*) [A-Z]`)
	// kermlMarker opens the examples the SysML specification writes in KerML.
	kermlMarker = regexp.MustCompile(`^//\s*KerML\b`)
	digits      = regexp.MustCompile(`^\d+$`)
)

// maxIndent is the deepest left margin an example starts at. The two-column
// notation tables render their textual column far to the right of it.
const maxIndent = 12

// Extract applies the extraction rules to the layout text of doc:
//
//   - The body starts at the first "7 …" heading on a page with a numeric page
//     number (the table of contents repeats the headings on roman-numbered pages).
//   - A block is a run of indented lines; a line at the left margin or a clause
//     heading ends it, and so does a blank line unless the block has more
//     opening than closing braces at that point (braces in comments and in
//     bullet items do not count). Running headers are skipped.
//   - A block is an example when it starts within maxIndent columns, lies in
//     one of doc.Clauses, carries no bullet, and some line ends in `;`, `{`, `}`
//     (before any trailing comment) or `*/`.
//   - Prose lines (eight or more words, none of `;`, `{`, `}`, `=`, `[`, `]`,
//     `:`) at the start or end of a block are the paragraph the layout glued to
//     the example and are dropped; a notation table (a line with guillemets, or
//     two lines with a column gap of twelve or more spaces) is excluded.
//   - An example whose code (outside comments) holds `...` is abridged by the
//     specification and is excluded, as is one whose braces do not balance.
//   - An example opening with a `// KerML` comment is in KerML notation.
func Extract(doc Document, text string) Result {
	var res Result
	var cur *candidate
	var comments commentStripper
	depth := 0
	clause := ""
	page := ""
	body := false
	counts := map[string]int{}

	flush := func() {
		if cur != nil {
			classify(doc, cur, counts, &res)
		}
		cur = nil
		depth = 0
	}

	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(strings.TrimPrefix(raw, "\f"), " \t\r")
		if m := pageHeader.FindStringSubmatch(line); m != nil {
			if res.Title == "" {
				res.Title = m[2]
			}
			if m[1] != "" {
				page = m[1]
			} else {
				page = m[3]
			}
			continue
		}
		if m := heading.FindStringSubmatch(line); m != nil {
			flush()
			clause = m[1]
			if clause == "7" && digits.MatchString(page) {
				body = true
			}
			continue
		}
		if !body {
			continue
		}
		if strings.TrimSpace(line) == "" {
			if cur != nil && depth > 0 {
				cur.lines = append(cur.lines, "")
			} else {
				flush()
			}
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 {
			flush()
			continue
		}
		if cur == nil {
			cur = &candidate{clause: clause, page: page, line: i + 1, indent: indent}
			comments = commentStripper{}
		}
		cur.lines = append(cur.lines, line)
		if !isBullet(line) {
			code := comments.strip(line)
			depth += strings.Count(code, "{") - strings.Count(code, "}")
		}
	}
	flush()
	return res
}

type candidate struct {
	clause string
	page   string
	line   int
	indent int
	lines  []string
}

// inClauses reports whether clause is one of, or nested under, the prefixes.
func inClauses(clause string, prefixes []string) bool {
	for _, p := range prefixes {
		if clause == p || strings.HasPrefix(clause, p+".") {
			return true
		}
	}
	return false
}

func classify(doc Document, c *candidate, counts map[string]int, res *Result) {
	for len(c.lines) > 0 && c.lines[len(c.lines)-1] == "" {
		c.lines = c.lines[:len(c.lines)-1]
	}
	if !inClauses(c.clause, doc.Clauses) || c.indent > maxIndent {
		return
	}
	terminated := false
	var comments commentStripper
	for _, l := range c.lines {
		t := strings.TrimSpace(l)
		if isBullet(l) {
			return
		}
		t = strings.TrimSpace(comments.strip(t))
		if strings.HasSuffix(t, ";") || strings.HasSuffix(t, "{") || strings.HasSuffix(t, "}") || strings.HasSuffix(l, "*/") {
			terminated = true
		}
	}
	if !terminated {
		return
	}
	exclude := func(reason string) {
		res.Excluded = append(res.Excluded, Excluded{
			Clause: c.clause, Page: c.page, Line: c.line, Reason: reason, First: strings.TrimSpace(c.lines[0]),
		})
	}
	if isTable(c.lines) {
		exclude("notation table")
		return
	}
	c.lines = trimProse(c.lines)
	if len(c.lines) == 0 {
		return
	}
	code := stripComments(strings.Join(c.lines, "\n"))
	if strings.Contains(code, "...") {
		exclude("abridged with ...")
		return
	}
	if strings.Count(code, "{") != strings.Count(code, "}") {
		exclude("unbalanced braces")
		return
	}
	lines := dedent(c.lines)
	ext := doc.Ext
	if kermlMarker.MatchString(lines[0]) {
		ext = "kerml"
	}
	counts[c.clause]++
	res.Blocks = append(res.Blocks, Block{
		Document: doc.Name,
		Clause:   c.clause,
		Page:     c.page,
		Line:     c.line,
		Lines:    lines,
		Ext:      ext,
		Index:    counts[c.clause],
	})
}

// isBullet reports a list item; bullets are prose, so their braces do not open a block.
func isBullet(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "•") || strings.HasPrefix(t, "◦") || strings.HasPrefix(t, "▪")
}

var columnGap = regexp.MustCompile(`\S {12,}\S`)

// isTable recognises the graphical-versus-textual notation tables: guillemet
// keywords, or two rows laid out in columns.
func isTable(lines []string) bool {
	gaps := 0
	for _, l := range lines {
		if strings.ContainsAny(l, "«»") {
			return true
		}
		if columnGap.MatchString(stripComments(l)) {
			gaps++
		}
	}
	return gaps >= 2
}

// isProse reports a sentence of running text: eight or more words without the
// punctuation every declaration carries.
func isProse(line string) bool {
	t := strings.TrimSpace(line)
	if strings.HasPrefix(t, "//") || strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "*") {
		return false
	}
	return len(strings.Fields(t)) >= 8 && !strings.ContainsAny(t, ";{}=[]:")
}

// trimProse drops the prose lines leading and trailing a block, and the blank
// lines they leave behind.
func trimProse(lines []string) []string {
	for len(lines) > 0 && (lines[0] == "" || isProse(lines[0])) {
		lines = lines[1:]
	}
	for len(lines) > 0 && (lines[len(lines)-1] == "" || isProse(lines[len(lines)-1])) {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// stripComments blanks `//` line comments and `/* */` block comments so that
// braces and ellipses inside them are not read as code.
func stripComments(s string) string {
	var cs commentStripper
	return cs.strip(s)
}

// commentStripper strips comments line by line, carrying an open block comment
// from one line to the next.
type commentStripper struct {
	inBlock bool
}

func (c *commentStripper) strip(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch {
		case c.inBlock:
			end := strings.Index(s[i:], "*/")
			if end < 0 {
				return b.String()
			}
			i += end + 2
			c.inBlock = false
		case strings.HasPrefix(s[i:], "//"):
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case strings.HasPrefix(s[i:], "/*"):
			c.inBlock = true
			i += 2
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// dedent removes the indentation every non-blank line shares.
func dedent(lines []string) []string {
	common := -1
	for _, l := range lines {
		if l == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " "))
		if common < 0 || n < common {
			common = n
		}
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if l != "" {
			l = l[common:]
		}
		out[i] = l
	}
	return out
}

// Path is the fixture's path under the corpus root: <document>/<clause>/<n>.<ext>.
func (b Block) Path() string {
	return b.Document + "/" + b.Clause + "/" + strconv.Itoa(b.Index) + "." + b.Ext
}
