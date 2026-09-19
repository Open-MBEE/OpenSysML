package xpect

import (
	"fmt"
	"regexp"
	"strings"
)

// Assertion kinds, spelled as the .xt files spell them.
const (
	kindErrors     = "errors"
	kindWarnings   = "warnings"
	kindNoErrors   = "noErrors"
	kindLinkedName = "linkedName"
	kindScope      = "scope"
	// kindExportedObjects declares what the file contributes to the index, as
	// `sysml::<Metaclass>: <qualified name>` lines.
	kindExportedObjects = "exportedObjects"
)

// resource is one entry of an XPECT_SETUP ResourceSet block: either the .xt
// file's own body or a path relative to the suite root.
type resource struct {
	ThisFile bool   `json:"thisFile,omitempty"`
	From     string `json:"from,omitempty"`
}

// expectation is one declared diagnostic inside an errors/warnings assertion:
// the message the pilot declares, and the source text it is declared at.
type expectation struct {
	Message string `json:"message"`
	At      string `json:"at,omitempty"`
}

// assertion is one XPECT comment. Region is the byte offset the assertion's
// target text is searched from — the start of the line after a line comment,
// the end of the note for a `//* ... */` block.
type assertion struct {
	Kind     string        `json:"kind"`
	Block    bool          `json:"block,omitempty"`
	Line     int           `json:"line"`
	Region   int           `json:"-"`
	Expect   []expectation `json:"expect,omitempty"`
	At       string        `json:"at,omitempty"`
	Expected string        `json:"expected,omitempty"`
	// Names is the comma-separated list an XPECT scope note declares.
	Names []string `json:"names,omitempty"`
	// Exported is the line-per-object list an XPECT exportedObjects note declares.
	Exported []string `json:"exported,omitempty"`
}

// xtFile is one parsed .xt test: the resource set its setup declares and the
// assertions in its body.
type xtFile struct {
	Path       string
	Language   string
	SetupClass string
	Resources  []resource
	Assertions []assertion
	Content    []byte
	// Masked is Content with every note blanked out, so an assertion's target
	// text is searched in model source only: an assertion is regularly followed
	// by another XPECT note quoting the same text.
	Masked []byte
	// Noted records, per byte of Content, whether a note covers that offset.
	Noted []bool
	// Problems records what could not be read. A file with problems is counted
	// as unparsed rather than compared.
	Problems []string
	// Ignored records XPECT-shaped text that opens no note, so that text this
	// harness does not run is reported rather than dropped.
	Ignored []string
}

var (
	setupClassRe = regexp.MustCompile(`XPECT_SETUP\s+(\S+)`)
	// The terminator is a whole token, so `END_SETUPX` does not terminate.
	endSetupRe  = regexp.MustCompile(`\bEND_SETUP\b`)
	xpectLineRe = regexp.MustCompile(`^[ \t]*//(\*?)[ \t]*XPECT[ \t]+([A-Za-z][A-Za-z0-9_]*)(.*)$`)
	// Resource entries: `ThisFile {}`, `File {from ="/p"}` and `File "p" {}`.
	fileFromRe = regexp.MustCompile(`\b(ThisFile\b|File\s*\{\s*from\s*=\s*"([^"]*)"|File\s*"([^"]*)"\s*\{)`)
	// The `at` text runs to the last quote of its line: Xpect does not escape
	// the quotes inside it, so `at "f(null, "", 1)"` is one text.
	quotedRe = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"(?:[ \t]*at[ \t]*"(.*)")?`)
	// The arrow is written `-->` or `--->`, with or without space around it.
	linkedNameRe = regexp.MustCompile(`^[ \t]*at[ \t]+(\S+?)[ \t]*-{2,}>[ \t]*(\S+)`)
	scopeAtRe    = regexp.MustCompile(`^[ \t]*at[ \t]+(\S+)`)
	// The declared name list follows a `---` fence or a `-->` arrow.
	scopeFenceRe = regexp.MustCompile(`-{2,}>?`)
	// XPECT-shaped text outside a `//`/`//*` note, so not an assertion here.
	strayRe = regexp.MustCompile(`XPECT[ \t]+([A-Za-z][A-Za-z0-9_]*)`)
	// A `//*` alone on its line opens a note whose keyword is on the next one.
	bareNoteRe = regexp.MustCompile(`^[ \t]*//\*[ \t]*$`)
	noteHeadRe = regexp.MustCompile(`^[ \t]*XPECT[ \t]+([A-Za-z][A-Za-z0-9_]*)(.*)$`)
)

// parseXT reads one .xt file: the XPECT_SETUP block, then every XPECT comment
// in the body. Anything it cannot read lands in Problems, never dropped.
func parseXT(path, language string, content []byte) xtFile {
	f := xtFile{Path: path, Language: language, Content: content}
	f.Masked, f.Noted = maskNotes(content)
	text := string(content)

	setupEnd, ok := f.parseSetup(text)
	if !ok {
		return f
	}
	f.parseAssertions(text, setupEnd)
	return f
}

// parseSetup records the test class and the declared resource set, and returns
// the offset the body starts at.
func (f *xtFile) parseSetup(text string) (int, bool) {
	match := setupClassRe.FindStringSubmatchIndex(text)
	if match == nil {
		f.Problems = append(f.Problems, "no XPECT_SETUP block")
		return 0, false
	}
	f.SetupClass = text[match[2]:match[3]]

	term := endSetupRe.FindStringIndex(text)
	if term == nil {
		f.Problems = append(f.Problems, "XPECT_SETUP block is not terminated by END_SETUP")
		return 0, false
	}
	end := term[0]
	// The setup lives in a `//* ... */` note; the body starts after it closes.
	bodyStart := term[1]
	if closer := strings.Index(text[bodyStart:], "*/"); closer >= 0 {
		bodyStart += closer + len("*/")
	}

	set, ok := braced(text[match[3]:end], "ResourceSet")
	if !ok {
		f.Problems = append(f.Problems, "no ResourceSet block in XPECT_SETUP")
		return bodyStart, false
	}
	for _, entry := range fileFromRe.FindAllStringSubmatch(set, -1) {
		if from := entry[2] + entry[3]; from != "" {
			f.Resources = append(f.Resources, resource{From: from})
			continue
		}
		f.Resources = append(f.Resources, resource{ThisFile: true})
	}
	if len(f.Resources) == 0 {
		f.Problems = append(f.Problems, "empty ResourceSet block")
	}
	return bodyStart, true
}

// braced returns the body of the named `name { ... }` block, brace-matched.
func braced(text, name string) (string, bool) {
	start := strings.Index(text, name)
	if start < 0 {
		return "", false
	}
	open := strings.Index(text[start:], "{")
	if open < 0 {
		return "", false
	}
	open += start
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[open+1 : i], true
			}
		}
	}
	return "", false
}

// parseAssertions walks the body line by line. A `//*` comment continues until
// the note closes, so the lines of a block assertion are consumed with it and
// can never be read as assertions of their own.
func (f *xtFile) parseAssertions(text string, from int) {
	lines, offsets := splitLines(text)
	for i := 0; i < len(lines); i++ {
		if offsets[i] < from {
			continue
		}
		match := xpectLineRe.FindStringSubmatch(lines[i])
		if match == nil && bareNoteRe.MatchString(lines[i]) && i+1 < len(lines) {
			if head := noteHeadRe.FindStringSubmatch(lines[i+1]); head != nil {
				i++
				match = []string{lines[i], "*", head[1], head[2]}
			}
		}
		if match == nil {
			if stray := strayRe.FindStringSubmatch(lines[i]); stray != nil {
				f.Ignored = append(f.Ignored, fmt.Sprintf(
					"line %d: `XPECT %s` opens no `//` or `//*` note, so this harness does not run it: %s",
					i+1, stray[1], strings.TrimSpace(lines[i])))
			}
			continue
		}
		block, kind, rest := match[1] == "*", match[2], match[3]
		a := assertion{Kind: kind, Block: block, Line: i + 1}

		last := i
		if block {
			body, end, ok := noteBody(lines, i, rest)
			if !ok {
				f.Problems = append(f.Problems,
					fmt.Sprintf("line %d: `//* XPECT %s` note is not terminated", i+1, kind))
				return
			}
			rest, last = body, end
		}
		if last+1 < len(lines) {
			a.Region = offsets[last+1]
		} else {
			a.Region = len(text)
		}
		if block {
			// The note's own text is not model source: search after it closes.
			if closer := strings.Index(text[offsets[last]:], "*/"); closer >= 0 {
				a.Region = offsets[last] + closer + len("*/")
			}
		}

		if err := a.parseBody(rest); err != nil {
			f.Problems = append(f.Problems, fmt.Sprintf("line %d: %v", i+1, err))
		}
		f.Assertions = append(f.Assertions, a)
		i = last
	}
}

// noteBody joins the lines of a `//* ... */` assertion, returning the joined
// text and the index of the line the note closes on.
func noteBody(lines []string, start int, rest string) (string, int, bool) {
	if closer := strings.Index(rest, "*/"); closer >= 0 {
		return rest[:closer], start, true
	}
	var b strings.Builder
	b.WriteString(rest)
	for i := start + 1; i < len(lines); i++ {
		if closer := strings.Index(lines[i], "*/"); closer >= 0 {
			b.WriteString("\n" + lines[i][:closer])
			return b.String(), i, true
		}
		b.WriteString("\n" + lines[i])
	}
	return "", start, false
}

// parseBody reads the arguments of one assertion, which differ per kind.
func (a *assertion) parseBody(rest string) error {
	switch a.Kind {
	case kindNoErrors:
		return nil
	case kindErrors, kindWarnings:
		// Both forms declare `"message" at "text"` items; the block form fences
		// them with `---`, the line form introduces them with `-->`.
		body := strings.TrimSpace(rest)
		body = strings.TrimSuffix(strings.TrimPrefix(body, "---"), "---")
		for _, item := range quotedRe.FindAllStringSubmatch(body, -1) {
			a.Expect = append(a.Expect, expectation{Message: unescape(item[1]), At: unescape(item[2])})
		}
		if len(a.Expect) == 0 {
			return fmt.Errorf("XPECT %s declares no expectation", a.Kind)
		}
		return nil
	case kindLinkedName:
		match := linkedNameRe.FindStringSubmatch(rest)
		if match == nil {
			return fmt.Errorf("XPECT linkedName is not `at <name> --> <qualified name>`")
		}
		a.At, a.Expected = match[1], match[2]
		return nil
	case kindScope:
		if match := scopeAtRe.FindStringSubmatch(rest); match != nil {
			// The anchor text may run straight into the fence: `at aliass---`.
			a.At = strings.TrimRight(match[1], "->")
		}
		a.Expected = strings.Join(strings.Fields(strings.Trim(strings.TrimSpace(rest), "-")), " ")
		a.Names = scopeNames(rest)
		if len(a.Names) == 0 {
			return fmt.Errorf("XPECT scope declares no name")
		}
		return nil
	case kindExportedObjects:
		a.Exported = exportedLines(rest)
		a.Expected = strings.Join(a.Exported, "; ")
		if len(a.Exported) == 0 {
			return fmt.Errorf("XPECT exportedObjects declares no object")
		}
		return nil
	default:
		return nil
	}
}

// scopeNames splits the declared name list of an XPECT scope note, which is
// fenced by `---` and follows the `at <text>` clause.
func scopeNames(rest string) []string {
	body := strings.TrimSpace(rest)
	if fence := scopeFenceRe.FindStringIndex(body); fence != nil {
		body = body[fence[1]:]
	}
	if fence := scopeFenceRe.FindStringIndex(body); fence != nil && strings.TrimSpace(body[fence[1]:]) == "" {
		body = body[:fence[0]]
	}
	var out []string
	for _, item := range strings.Split(body, ",") {
		if name := strings.Join(strings.Fields(item), ""); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// exportedLines splits the object list of an XPECT exportedObjects note, one
// `sysml::<Metaclass>: <qualified name>` per line inside a `---` fence.
func exportedLines(rest string) []string {
	var out []string
	for _, line := range strings.Split(rest, "\n") {
		line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "-"))
		if line == "" {
			continue
		}
		out = append(out, strings.Join(strings.Fields(line), " "))
	}
	return out
}

// unescape undoes the backslash escapes a declared message is written with.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	r := strings.NewReplacer(`\"`, `"`, `\\`, `\`, `\n`, "\n", `\t`, "\t")
	return r.Replace(s)
}

// maskNotes returns content with the text of every note replaced by spaces,
// keeping every offset, and a per-byte record of which offsets a note covers.
// KerML and SysML write notes as `//` to end of line and `//* ... */`, and both
// forms carry XPECT assertions.
func maskNotes(content []byte) ([]byte, []bool) {
	out := make([]byte, len(content))
	copy(out, content)
	noted := make([]bool, len(content))
	blank := func(from, to int) {
		for i := from; i < to && i < len(out); i++ {
			noted[i] = true
			if out[i] != '\n' && out[i] != '\r' {
				out[i] = ' '
			}
		}
	}
	for i := 0; i+1 < len(content); i++ {
		if content[i] != '/' {
			continue
		}
		var end int
		switch {
		case i+2 < len(content) && content[i+1] == '/' && content[i+2] == '*':
			end = indexPast(content, i+3, "*/")
		case content[i+1] == '/':
			end = indexPast(content, i+2, "\n")
		case content[i+1] == '*':
			end = indexPast(content, i+2, "*/")
		default:
			continue
		}
		blank(i, end)
		i = end - 1
	}
	return out, noted
}

// indexPast returns the offset just past sep at or after from, or the end.
func indexPast(content []byte, from int, sep string) int {
	if from >= len(content) {
		return len(content)
	}
	if at := strings.Index(string(content[from:]), sep); at >= 0 {
		return from + at + len(sep)
	}
	return len(content)
}

// splitLines returns the lines of text with each line's byte offset.
func splitLines(text string) ([]string, []int) {
	var lines []string
	var offsets []int
	start := 0
	for i := 0; i <= len(text); i++ {
		if i == len(text) || text[i] == '\n' {
			line := text[start:i]
			lines = append(lines, strings.TrimSuffix(line, "\r"))
			offsets = append(offsets, start)
			start = i + 1
		}
	}
	return lines, offsets
}
