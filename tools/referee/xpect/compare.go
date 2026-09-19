package xpect

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	corediag "github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Verdicts. Agreement is strict: the declared severity at the declared offset,
// with the declared message. Tolerance records what a weaker rule would have
// accepted, and never turns a disagreement into an agreement.
const (
	verdictAgree = "agree"
	// verdictWordingOnly is the same rule about the same element in our own
	// words; it is counted inside agreement, with its own sub-count.
	verdictWordingOnly    = "wording-only"
	verdictDisagree       = "disagree"
	verdictUnlocated      = "unlocated"
	verdictNotAdjudicated = "not-adjudicated"
)

// Tolerances, reported as secondary columns beside a strict disagreement.
const (
	toleranceNone     = ""
	toleranceMessage  = "same-location"     // right severity and offset, other wording
	toleranceLine     = "same-line"         // right severity on the declared line
	toleranceSeverity = "severity-differs"  // a diagnostic there, of the other severity
	toleranceAnywhere = "elsewhere-in-file" // right severity, but not near the declaration
)

// Scope tolerances. A scope assertion declares a whole set, so a near-miss is
// classified by which half of the set differs.
const (
	toleranceScopeSpelling = "other-paths"       // every declared element, some under another path
	toleranceScopeExtra    = "extra-names"       // every declared name, and more besides
	toleranceScopeMissing  = "missing-names"     // declared names we do not offer at all
	toleranceScopeBoth     = "missing-and-extra" // both halves differ
	// library-names: differs only in names inherited from the standard library,
	// which a fixture sees only if its resource set loads it.
	toleranceScopeLibrary = "library-names"
)

// row is one adjudicated expectation: one item of an errors/warnings assertion,
// or one whole noErrors/linkedName assertion.
type row struct {
	Kind string `json:"kind"`
	// Block records a `//* ... */` note, which the brief's line-anchored census
	// of the suites does not count.
	Block     bool   `json:"block,omitempty"`
	Line      int    `json:"line"`
	At        string `json:"at,omitempty"`
	Verdict   string `json:"verdict"`
	Tolerance string `json:"tolerance,omitempty"`
	// Rule names the rule a wording-only row shows both sides stating.
	Rule     string `json:"rule,omitempty"`
	Declared string `json:"declared,omitempty"`
	Actual   string `json:"ours,omitempty"`
	// Names counts a scope assertion's set differences.
	Names *scopeCounts `json:"names,omitempty"`
}

// fileResult is one .xt file's adjudication.
type fileResult struct {
	Path       string `json:"path"`
	SetupClass string `json:"setupClass"`
	// Expectations and Agree summarize Rows, which the report prunes to the
	// rows that are not agreements.
	Expectations int      `json:"expectations"`
	Agree        int      `json:"agree"`
	Rows         []row    `json:"rows,omitempty"`
	Problems     []string `json:"problems,omitempty"`
	// Ignored lists XPECT-shaped text that opens no note, so it is not run.
	Ignored []string `json:"ignored,omitempty"`
	// Missing lists declared resources that are absent from the download.
	Missing []string `json:"missing,omitempty"`
	// Foreign counts diagnostics dropped as another resource's, see elsewhere.
	Foreign int `json:"foreignDiagnostics,omitempty"`
}

// diag is one of our diagnostics, reduced to what an assertion declares.
type diag struct {
	Offset   int
	Line     int
	Severity string
	Message  string
}

// foreignDiagnostic reports whether a diagnostic lands outside the file's model
// text — inside a note or past its end — so it was raised for another declared
// resource and is not the file's to answer for.
func foreignDiagnostic(f xtFile, offset int) bool {
	if offset < 0 || offset >= len(f.Noted) {
		return true
	}
	return f.Noted[offset]
}

// compareFile loads the resource set the file's setup declares and adjudicates
// every assertion in it.
func compareFile(suiteDir string, f xtFile, libs *libraryCache) fileResult {
	res := fileResult{Path: f.Path, SetupClass: f.SetupClass, Problems: f.Problems, Ignored: f.Ignored}
	if len(f.Problems) > 0 {
		return res
	}

	set := loadResourceSet(suiteDir, f, libs)
	ws, libraryRoots := set.ws, set.libraryRoots
	res.Missing = set.missing
	main := strings.TrimSuffix(f.Path, ".xt")

	src := squeeze(f.Masked)
	lines := source.New(main, f.Content).Lines()
	var diags []diag
	for _, d := range ws.Diagnostics(main) {
		if foreignDiagnostic(f, d.Span.Offset) {
			res.Foreign++
			continue
		}
		diags = append(diags, diag{
			Offset:   d.Span.Offset,
			Line:     lines.PosAt(d.Span.Offset).Line,
			Severity: severityName(d.Severity),
			Message:  d.Message,
		})
	}

	// Xpect matches every issue against the expectations' regions and fails the
	// file on the ones left over, so an error another expectation declares is
	// not the file's residue.
	consumed := map[int]bool{}
	for _, a := range f.Assertions {
		if a.Kind != kindErrors && a.Kind != kindNoErrors {
			continue
		}
		if l := consumedLine(f.Content, lines, a.Region); l > 0 {
			consumed[l] = true
		}
	}

	subject := adjudicand{
		ws: ws, main: main, diags: diags, lines: lines,
		src: src, libraryRoots: libraryRoots, consumed: consumed,
	}
	for _, a := range f.Assertions {
		own := consumedLine(f.Content, lines, a.Region)
		res.Rows = append(res.Rows, subject.adjudicate(a, own)...)
	}
	return res
}

// consumedLine is the line an assertion's issues are matched in: the first line
// of model text after its note.
func consumedLine(content []byte, lines *source.LineIndex, region int) int {
	for i := region; i < len(content); i++ {
		switch content[i] {
		case ' ', '\t', '\r', '\n':
		default:
			return lines.PosAt(i).Line
		}
	}
	return -1
}

// adjudicand is one file's loaded resources and diagnostics, against which its
// assertions are adjudicated.
type adjudicand struct {
	ws           *model.Workspace
	main         string
	diags        []diag
	lines        *source.LineIndex
	src          squeezed
	libraryRoots []string
	consumed     map[int]bool
}

// adjudicate turns one assertion into its rows.
func (s adjudicand) adjudicate(a assertion, own int) []row {
	ws, main, diags, lines, src := s.ws, s.main, s.diags, s.lines, s.src
	libraryRoots, consumed := s.libraryRoots, s.consumed
	switch a.Kind {
	case kindErrors, kindWarnings:
		want := "error"
		if a.Kind == kindWarnings {
			want = "warning"
		}
		rows := make([]row, 0, len(a.Expect))
		for _, item := range a.Expect {
			rows = append(rows, diagnosticRow(a, item, want, diags, lines, src))

		}
		return rows
	case kindNoErrors:
		var errs []diag
		for _, d := range diags {
			if d.Severity != "error" {
				continue
			}
			// The assertion's own line is the one it declares clean; elsewhere
			// only an error no expectation declares is its to answer for.
			if d.Line != own && consumed[d.Line] {
				continue
			}
			errs = append(errs, d)
		}
		r := row{Kind: a.Kind, Block: a.Block, Line: a.Line, Verdict: verdictAgree, Declared: "no error the file does not declare"}
		if len(errs) > 0 {
			r.Verdict = verdictDisagree
			r.Actual = fmt.Sprintf("%d error(s), first: line %d: %s", len(errs), errs[0].Line, errs[0].Message)
		}
		return []row{r}
	case kindLinkedName:
		return []row{linkedNameRow(ws, main, a, src)}
	case kindScope:
		return []row{scopeRow(ws, main, a, src, libraryRoots)}
	case kindExportedObjects:
		return []row{exportedObjectsRow(ws, main, a)}
	default:
		return []row{{
			Kind: a.Kind, Block: a.Block, Line: a.Line, At: a.At, Verdict: verdictNotAdjudicated,
			Declared: a.Expected,
			Actual:   fmt.Sprintf("XPECT %s is read but not adjudicated by this harness", a.Kind),
		}}
	}
}

// diagnosticRow adjudicates one declared diagnostic.
func diagnosticRow(a assertion, item expectation, want string, diags []diag, lines *source.LineIndex, src squeezed) row {
	r := row{Kind: a.Kind, Block: a.Block, Line: a.Line, At: item.At, Declared: fmt.Sprintf("%s: %q", want, item.Message)}

	offset, _, ok := src.locate(a.Region, item.At)
	if !ok {
		r.Verdict = verdictUnlocated
		r.Actual = fmt.Sprintf("the declared text %q does not occur after the assertion", item.At)
		return r
	}
	line := lines.PosAt(offset).Line

	var atOffset, atLine, elsewhere, otherSeverity []diag
	for _, d := range diags {
		if d.Severity != want {
			if d.Offset == offset || d.Line == line {
				otherSeverity = append(otherSeverity, d)
			}
			continue
		}
		switch {
		case d.Offset == offset:
			atOffset = append(atOffset, d)
		case d.Line == line:
			atLine = append(atLine, d)
		default:
			elsewhere = append(elsewhere, d)
		}
	}

	for _, d := range atOffset {
		if sameMessage(d.Message, item.Message) {
			r.Verdict = verdictAgree
			r.Actual = fmt.Sprintf("line %d offset %d: %s", d.Line, d.Offset, d.Message)
			return r
		}
	}
	for _, d := range atOffset {
		if class, ok := wordingOnly(item.Message, d.Message); ok {
			r.Verdict = verdictWordingOnly
			r.Rule = class
			r.Actual = fmt.Sprintf("line %d offset %d: %s", d.Line, d.Offset, d.Message)
			return r
		}
	}
	r.Verdict = verdictDisagree
	switch {
	case len(atOffset) > 0:
		r.Tolerance = toleranceMessage
		r.Actual = fmt.Sprintf("line %d offset %d: %s", atOffset[0].Line, atOffset[0].Offset, atOffset[0].Message)
	case len(atLine) > 0:
		r.Tolerance = toleranceLine
		r.Actual = fmt.Sprintf("line %d offset %d: %s", atLine[0].Line, atLine[0].Offset, atLine[0].Message)
	case len(otherSeverity) > 0:
		r.Tolerance = toleranceSeverity
		r.Actual = fmt.Sprintf("no %s at line %d, but a %s: line %d offset %d: %s", want, line,
			otherSeverity[0].Severity, otherSeverity[0].Line, otherSeverity[0].Offset, otherSeverity[0].Message)
	case len(elsewhere) > 0:
		r.Tolerance = toleranceAnywhere
		r.Actual = fmt.Sprintf("no %s at line %d; nearest is line %d: %s", want, line, elsewhere[0].Line, elsewhere[0].Message)
	default:
		r.Actual = fmt.Sprintf("no %s anywhere in the file", want)
	}
	return r
}

// linkedNameRow resolves the reference the assertion points at and compares its
// qualified name with the declared one.
func linkedNameRow(ws *model.Workspace, main string, a assertion, src squeezed) row {
	r := row{Kind: a.Kind, Block: a.Block, Line: a.Line, At: a.At, Declared: a.Expected}

	offset, end, ok := src.locate(a.Region, a.At)
	if !ok {
		r.Verdict = verdictUnlocated
		r.Actual = fmt.Sprintf("the declared text %q does not occur after the assertion", a.At)
		return r
	}

	doc := ws.Document(main)
	if doc == nil {
		r.Verdict = verdictUnlocated
		r.Actual = "the file did not parse into a document"
		return r
	}
	ref, part, ok := referenceAt(resolve.References(doc.AST, doc.Scope), offset, end)
	if !ok {
		// The text is there but we index no reference at it: either we do not
		// parse the construct or we do not treat it as a name reference.
		r.Verdict = verdictDisagree
		r.Actual = fmt.Sprintf("we index no name reference at offset %d", offset)
		return r
	}

	var sym *symbols.Symbol
	if part == len(ref.QN.Parts)-1 {
		sym, _ = ws.ResolveReferenceInDoc(main, ref)
	} else if segs := ws.ResolveReferenceSegmentsInDoc(main, ref); part < len(segs) {
		sym = segs[part]
	}
	if sym == nil {
		r.Verdict = verdictDisagree
		r.Actual = "unresolved"
		return r
	}

	actual := dotted(symbols.FQNOf(sym))
	r.Actual = actual
	r.Verdict = verdictDisagree
	if actual == a.Expected {
		r.Verdict = verdictAgree
	}
	return r
}

// referenceAt picks the segment a declared `at` text covers: the assertion
// names a whole qualified name, so the segment it asks about is the last one
// ending inside [offset, end). Sorting keeps the choice deterministic when a
// name is nested in another reference's chain.
func referenceAt(refs []resolve.Reference, offset, end int) (resolve.Reference, int, bool) {
	type candidate struct {
		ref  resolve.Reference
		part int
	}
	var found []candidate
	for _, ref := range refs {
		if ref.QN == nil || len(ref.QN.Parts) == 0 {
			continue
		}
		for i, seg := range ref.QN.Parts {
			// A quoted name's span covers the quotes the declared text omits.
			if seg.Span.Offset != offset && seg.Span.Offset != offset-1 {
				continue
			}
			// The anchor starts at this segment and runs to end, which may
			// cover several segments of the same name.
			last := i
			for j := i; j < len(ref.QN.Parts); j++ {
				if ref.QN.Parts[j].Span.End() <= end {
					last = j
				}
			}
			found = append(found, candidate{ref, last})
			break
		}
	}
	if len(found) == 0 {
		return resolve.Reference{}, 0, false
	}
	sort.SliceStable(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if a.ref.QN.Span().Offset != b.ref.QN.Span().Offset {
			return a.ref.QN.Span().Offset < b.ref.QN.Span().Offset
		}
		if len(a.ref.QN.Parts) != len(b.ref.QN.Parts) {
			return len(a.ref.QN.Parts) > len(b.ref.QN.Parts)
		}
		return a.part < b.part
	})
	return found[0].ref, found[0].part, true
}

// squeezed is model source with the notes blanked out and all whitespace
// dropped, keeping each kept byte's original offset. Declared target texts are
// matched against it because the suites space them freely: `attribute un : A::p;`
// is declared for the source `attribute un: A::p;`.
type squeezed struct {
	Original []byte
	Text     string
	Offsets  []int
}

func squeeze(content []byte) squeezed {
	s := squeezed{Original: content, Offsets: make([]int, 0, len(content))}
	var b strings.Builder
	for i := 0; i < len(content); i++ {
		if isSpace(content[i]) {
			continue
		}
		b.WriteByte(content[i])
		s.Offsets = append(s.Offsets, i)
	}
	s.Text = b.String()
	return s
}

// locate returns the original span of the first occurrence of text at or after
// from. An occurrence inside a longer identifier does not count.
func (s squeezed) locate(from int, text string) (int, int, bool) {
	for _, span := range s.locateAll(from, text) {
		if span.whole {
			return span.begin, span.end, true
		}
	}
	return 0, 0, false
}

// match is one occurrence of an `at` text, whole recording whether it is the
// entire identifier rather than the start of a longer one.
type match struct {
	begin, end int
	whole      bool
}

// locateAll returns every occurrence of text that starts an identifier at or
// after from, in source order.
func (s squeezed) locateAll(from int, text string) []match {
	start := sort.SearchInts(s.Offsets, from)
	if text == "" {
		// No `at` clause: the assertion targets the source it precedes.
		if start >= len(s.Offsets) {
			return nil
		}
		return []match{{s.Offsets[start], s.Offsets[start] + 1, true}}
	}
	want := strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}), "")
	if want == "" {
		return nil
	}
	var out []match
	for i := start; i+len(want) <= len(s.Text); i++ {
		if s.Text[i:i+len(want)] != want {
			continue
		}
		begin, end := s.Offsets[i], s.Offsets[i+len(want)-1]+1
		if identChar(want[0]) && begin > 0 && identChar(s.Original[begin-1]) {
			continue
		}
		whole := !identChar(want[len(want)-1]) || end >= len(s.Original) || !identChar(s.Original[end])
		out = append(out, match{begin, end, whole})
	}
	return out
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func identChar(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// rootPackageRe matches an unindented package declaration, which in a library
// file is one of its root packages.
var rootPackageRe = regexp.MustCompile(`(?m)^(?:standard\s+)?(?:library\s+)?package\s+('[^']*'|[A-Za-z_]\w*)`)

// rootPackagesIn reads the root packages a declared library resource declares,
// which is what names its contents are reachable under.
func rootPackagesIn(content []byte) []string {
	var roots []string
	for _, m := range rootPackageRe.FindAllStringSubmatch(string(content), -1) {
		roots = append(roots, strings.Trim(m[1], "'"))
	}
	return roots
}

// isLibrary reports whether a declared resource is one of the suite's copies of
// a standard library file.
func isLibrary(from string) bool {
	return strings.HasPrefix(from, "/library")
}

// sameMessage compares a declared message with ours, ignoring whitespace and
// the trailing period the pilot's messages carry.
func sameMessage(ours, declared string) bool {
	norm := func(s string) string {
		return strings.TrimSuffix(strings.Join(strings.Fields(s), " "), ".")
	}
	return norm(ours) == norm(declared)
}

// dotted rewrites our `::`-separated qualified name in the pilot's notation.
func dotted(fqn string) string {
	return strings.ReplaceAll(fqn, "::", ".")
}

func severityName(s corediag.Severity) string {
	return strings.ToLower(s.String())
}

func dedupe(in []string) []string {
	var out []string
	for i, s := range in {
		if i == 0 || in[i-1] != s {
			out = append(out, s)
		}
	}
	return out
}
