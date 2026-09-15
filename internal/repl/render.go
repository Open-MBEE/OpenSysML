package repl

import (
	"fmt"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/width"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Result is the outcome of one Submit: the top-level members of the session's
// documents (for the success summary), the names this submission declared, and
// any analysis diagnostics over the whole buffer.
type Result struct {
	Members     []Member            // top-level members of the session documents, in buffer order
	Declared    []string            // names introduced by THIS submission
	Diagnostics []passes.Diagnostic // eager analysis over the whole buffer
	Source      string              // the full joined <repl> content (Task 6 caret rendering)
	Offset      int                 // byte offset in Source where THIS submission begins
	Origins     []Origin            // the files of THIS submission, in buffer order
	Notices     []string            // side effects of the submission, e.g. a debugging session it ended

	// Blocked names the unresolved error that stopped the deeper checks from
	// running over this submission, nil when they ran or when the session already
	// reported that error.
	Blocked *blocker

	// own locates this submission inside Source when it was merged into text
	// already in the buffer, which is not one tail region. Empty for a
	// submission appended whole, where everything from Offset on is its own.
	own []source.Span

	// masked locates the submissions kept out of the analyzed buffer, whose
	// findings gated no validation tier.
	masked []source.Span
}

// Member is one top-level member of a session document; Offset is where the
// member begins in the buffer, a loaded file's document having offsets of its own.
type Member struct {
	Node   ast.Node
	Offset int
	// scope is the root scope of the document declaring the member.
	scope *symbols.Scope
}

// Origin locates one file of a submission in the buffer, so a diagnostic is
// reported against that file and its own line numbering.
type Origin struct {
	Name   string
	Offset int
}

// locate returns the file a buffer offset belongs to and the offset that file
// starts at. An offset outside the files of this submission belongs to the
// submission as a whole, which is reported without a file name.
func (r Result) locate(offset int) (string, int) {
	for i, o := range r.Origins {
		if offset < o.Offset {
			continue
		}
		if i+1 == len(r.Origins) || offset < r.Origins[i+1].Offset {
			return o.Name, o.Offset
		}
	}
	return "", r.Offset
}

// lineOf is the 1-based buffer line a byte offset falls on.
func (r Result) lineOf(offset int) int {
	if offset > len(r.Source) {
		offset = len(r.Source)
	}
	return strings.Count(r.Source[:offset], "\n") + 1
}

// diagLocation names the file a diagnostic came from and the buffer line that
// file — or, for a prompt submission, the submission — starts on.
func (r Result) diagLocation(offset int) (string, int) {
	name, start := r.locate(offset)
	return name, r.lineOf(start)
}

// mine reports whether a span belongs to the submission just made rather than
// to an earlier one still sitting in the buffer.
func (r Result) mine(span source.Span) bool {
	if len(r.own) == 0 {
		return span.Offset >= r.Offset
	}
	for _, o := range r.own {
		if span.Offset >= o.Offset && span.Offset < o.End() {
			return true
		}
	}
	return false
}

// holdsMine reports whether a span overlaps this submission, which is what
// credits a merged addition to the declaration it was added to, and a whole
// snippet's span to every declaration inside it.
func (r Result) holdsMine(span source.Span) bool {
	if len(r.own) == 0 {
		return span.Offset >= r.Offset
	}
	for _, o := range r.own {
		if o.Offset >= span.Offset && o.Offset < span.End() {
			return true
		}
		if span.Offset >= o.Offset && span.Offset < o.End() {
			return true
		}
	}
	return false
}

// renderSummary returns one accepted line per top-level member: "✓ <kind> <name>".
func renderSummary(members []Member) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		if line := renderMember(m.Node); line != "" {
			out = append(out, "✓ "+line)
		}
	}
	return out
}

// renderMember maps a top-level member (possibly wrapped in a Membership) to a
// "<kind> <name>" summary, or "" for members that carry no useful summary.
func renderMember(m ast.Node) string {
	node := m
	if mem, ok := m.(*ast.Membership); ok {
		node = mem.Member
	}
	switch d := node.(type) {
	case *ast.Package:
		return "package " + nameOrAnon(d.Ident)
	case *ast.Namespace:
		return "namespace " + nameOrAnon(d.Ident)
	case *ast.Alias:
		return "alias " + nameOrAnon(d.Ident)
	case *ast.Import:
		return "import " + importTarget(d)
	case *ast.Dependency:
		return "dependency " + nameOrAnon(d.Ident)
	case *ast.Comment:
		return "comment"
	case *ast.RelationshipMember:
		return ast.Notation(d) + " " + nameOrAnon(d.Ident)
	case *ast.Definition:
		return ast.Notation(d) + " " + nameOrAnon(d.Ident)
	case *ast.Usage:
		return ast.Notation(d) + " " + nameOrAnon(d.Ident)
	default:
		return ""
	}
}

func nameOrAnon(id ast.Identification) string {
	if id.Name != "" {
		return lexer.NameText(id.Name)
	}
	if id.ShortName != "" {
		return "<" + lexer.NameText(id.ShortName) + ">"
	}
	return "<anonymous>"
}

// importTarget echoes what an import names, wildcards included, so the
// confirmation matches what was typed.
func importTarget(imp *ast.Import) string {
	name := qnString(imp.Imported)
	switch {
	case imp.Kind == ast.ImportNamespace && imp.IsRecursive:
		return name + "::*::**"
	case imp.IsRecursive:
		return name + "::**"
	case imp.Kind == ast.ImportNamespace:
		return name + "::*"
	}
	return name
}

func qnString(qn *ast.QualifiedName) string {
	if qn == nil {
		return "<?>"
	}
	parts := make([]string, len(qn.Parts))
	for i, p := range qn.Parts {
		parts[i] = p.Text
	}
	return strings.Join(parts, "::")
}

// renderDiagnostics formats each diagnostic as a two-line block:
//
//	<line>:<col>: <severity>: <message>
//	    <source line>
//	    <caret span>
//
// locate maps a diagnostic's buffer offset to the file it came from and the
// buffer line that file starts on, so a block names its file and counts lines
// from what the user submitted rather than from the top of the accumulated
// buffer; pass wholeBuffer to number against the buffer instead. When origin is
// set each block also carries the pass that produced the diagnostic.
//
// Columns and carets are counted in printed cells, so a line with multi-byte
// runes before the finding still points at it; the LSP server owns UTF-16
// correctness separately.
func renderDiagnostics(diags []passes.Diagnostic, src string, locate func(offset int) (string, int), origin bool) []string {
	if len(diags) == 0 {
		return nil
	}
	sf := source.New(docName, []byte(src))
	lineIndex := sf.Lines()
	lines := strings.Split(src, "\n")
	var out []string
	for _, d := range diags {
		p := lineIndex.PosAt(d.Span.Offset)
		file, baseLine := locate(d.Span.Offset)
		where := ""
		if file != "" {
			where = file + ":"
		}
		srcLine := ""
		if p.Line-1 >= 0 && p.Line-1 < len(lines) {
			srcLine = lines[p.Line-1]
		}
		col := p.Col
		if p.Col-1 <= len(srcLine) {
			col = displayWidth(srcLine[:p.Col-1]) + 1
		}
		head := fmt.Sprintf("%s%d:%d: %s: %s", where, p.Line-baseLine+1, col, d.Severity.String(), d.Message)
		if origin {
			head += fmt.Sprintf(" [%s]", diagOrigin(d))
		}
		out = append(out, head)
		if p.Line-1 >= 0 && p.Line-1 < len(lines) {
			out = append(out, srcLine)
			out = append(out, caretLine(srcLine, p.Col-1, d.Span.Len))
		}
	}
	return out
}

// diagOrigin names the pass and code behind a diagnostic, for debug output.
func diagOrigin(d passes.Diagnostic) string {
	parts := make([]string, 0, 2)
	for _, p := range []string{d.Source, d.Code} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return "unattributed"
	}
	return strings.Join(parts, "/")
}

// exprError reports msg against a one-line expression the way a declaration
// diagnostic is reported: position, source echo and caret. base is the offset
// the expression starts at in the text span was measured in.
// The reported column counts printed cells, not bytes, so it agrees with the
// caret under the echo.
func exprError(expr, msg string, span source.Span, base int) error {
	start := span.Offset - base
	if start < 0 {
		start = 0
	}
	if start > len(expr) {
		start = len(expr)
	}
	return fmt.Errorf("1:%d: %s\n%s\n%s", displayWidth(expr[:start])+1, msg, expr, caretLine(expr, start, span.Len))
}

// caretLine builds "   ^~~~" under the span starting at byte offset start of
// line, measured in printed cells so multi-byte runes stay aligned.
func caretLine(line string, start, spanLen int) string {
	if start < 0 {
		start = 0
	}
	if start > len(line) {
		start = len(line)
	}
	end := start + spanLen
	if end > len(line) {
		end = len(line)
	}
	width := 0
	if end > start {
		width = displayWidth(line[start:end])
	}
	if width < 1 {
		width = 1
	}
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", displayWidth(line[:start])))
	b.WriteByte('^')
	if width > 1 {
		b.WriteString(strings.Repeat("~", width-1))
	}
	return b.String()
}

// displayWidth is the number of terminal cells s occupies: two for the East
// Asian wide and fullwidth runes, one for everything else, and none for the
// combining marks that render on the rune before them.
func displayWidth(s string) int {
	cells := 0
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r):
		case width.LookupRune(r).Kind() == width.EastAsianWide,
			width.LookupRune(r).Kind() == width.EastAsianFullwidth:
			cells += 2
		default:
			cells++
		}
	}
	return cells
}

// renderResult produces the printable lines for a submission at the given
// verbosity: the diagnostics that verbosity admits, the summary of what it
// declared, and last the notices it caused, which read as consequences of the
// summary above them. A submission that failed to analyse gets diagnostics
// instead of a summary, since it declared nothing usable.
func renderResult(r Result, v Verbosity) []string {
	found, declared := renderSplit(r, v)
	return append(found, declared...)
}

// renderSplit renders a submission as renderResult does, keeping what the
// analysis found apart from what the submission declared, so a caller outside
// the prompt can send the two to different streams.
func renderSplit(r Result, v Verbosity) (found, declared []string) {
	if v >= VerbosityDebug {
		// Everything the analysis produced over the whole buffer, at
		// buffer-absolute positions, plus where this submission landed in it.
		found = []string{fmt.Sprintf("[debug] submission at buffer line %d; %d diagnostic(s) over the whole buffer",
			r.baseLine(), len(r.Diagnostics))}
		found = append(found, renderDiagnostics(r.Diagnostics, r.Source, wholeBuffer, true)...)
		return found, append(renderSummary(r.Members), r.Notices...)
	}
	diags := scopedDiagnostics(r, v)
	found = renderDiagnostics(diags, r.Source, r.diagLocation, false)
	if hasError(diags) {
		return found, r.Notices
	}
	// A validation tier is skipped once a lower tier errors anywhere in the
	// buffer, so a clean report on this submission would otherwise read as a
	// full check when the deeper passes never ran.
	if note := r.Blocked.note(); note != "" {
		found = append(found, note)
	}
	return found, append(renderSummary(r.ownMembers()), r.Notices...)
}

// renderSyntax renders the syntax errors of this submission, which are about the
// text just read rather than about the analysis of the model as a whole: a load
// that defers the analysis still says why a file could not be read.
func renderSyntax(r Result, v Verbosity) []string {
	// A finding about the notation is no reason a file could not be read, and the
	// analysis this load defers reports it, so reporting it here would report it twice.
	var diags []passes.Diagnostic
	for _, d := range scopedDiagnostics(r, v) {
		if d.Source == "syntax" && d.Blocking() {
			diags = append(diags, d)
		}
	}
	if len(diags) == 0 {
		return nil
	}
	return renderDiagnostics(diags, r.Source, r.diagLocation, false)
}

// scopedDiagnostics keeps the diagnostics of this submission that the verbosity
// admits: errors always, warnings and below only above quiet.
func scopedDiagnostics(r Result, v Verbosity) []passes.Diagnostic {
	var out []passes.Diagnostic
	for _, d := range r.Diagnostics {
		if !r.mine(d.Span) {
			continue
		}
		if d.Severity != passes.SeverityError && v <= VerbosityQuiet {
			continue
		}
		out = append(out, d)
	}
	return out
}

// blocker is the unresolved error that stopped the higher validation tiers from
// running over a submission: the buffer line it was reported on, what it said,
// and how many further errors sit elsewhere in the buffer.
type blocker struct {
	offset  int
	line    int
	message string
	more    int

	// reported records the blockage as named, and is called when the note is
	// emitted rather than when it is prepared: a rendering that leaves the note
	// out has not told the user anything to be quiet about afterwards.
	reported func()
}

// key identifies what is blocking the checks, so an unchanged blockage is
// reported once rather than on every submission made under it, while a new error
// joining it is reported again.
func (b *blocker) key() string {
	return fmt.Sprintf("%d:%d:%s", b.more, b.offset, b.message)
}

// note warns that a clean report is not a full check, naming the line the
// unresolved error is on so it can be gone back to rather than only known about.
// The error itself is not re-reported: it is this submission's report, and the
// finding belongs to the line it was written on.
func (b *blocker) note() string {
	if b == nil {
		return ""
	}
	if b.reported != nil {
		b.reported()
	}
	if b.more > 0 {
		return fmt.Sprintf("note: deeper checks may not have run here: the error on buffer line %d is unresolved, with %s elsewhere in the buffer (see them with -debug)",
			b.line, countOf(b.more, "error", "errors"))
	}
	return fmt.Sprintf("note: deeper checks may not have run here: the error on buffer line %d is unresolved (see it with -debug)", b.line)
}

// blockedBy reports the unresolved error that stopped the deeper checks from
// running over this submission: a standing error is named on the first
// submission whose report says so, not on every one after it.
func (s *Session) blockedBy(r Result) *blocker {
	b := r.analysisBlocked()
	if b == nil {
		s.notedBlocker.record("")
		return nil
	}
	key := b.key()
	if key == s.notedBlocker.reportedKey() {
		return nil
	}
	b.reported = func() { s.notedBlocker.record(key) }
	return b
}

// blockerNote is the blockage a session has already named. Rendering is what
// names it, and runs after the submission released the session lock, so this
// carries a lock of its own.
type blockerNote struct {
	mu  sync.Mutex
	key string
}

func (n *blockerNote) reportedKey() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.key
}

func (n *blockerNote) record(key string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.key = key
}

// analysisBlocked returns the error outside this submission that stopped the
// higher validation tiers from running over it, nil when there is none.
func (r Result) analysisBlocked() *blocker {
	var first *blocker
	for _, d := range r.Diagnostics {
		if !d.Blocking() || r.mine(d.Span) || r.isMasked(d.Span) {
			continue
		}
		if first != nil {
			first.more++
			continue
		}
		first = &blocker{offset: d.Span.Offset, line: r.lineOf(d.Span.Offset), message: d.Message}
	}
	return first
}

// isMasked reports whether a span falls in a submission that was kept out of
// the analyzed buffer, so its errors blocked nothing.
func (r Result) isMasked(span source.Span) bool {
	for _, m := range r.masked {
		// End() included: a submission that does not close its own text is
		// reported at its end as often as inside it.
		if span.Offset >= m.Offset && span.Offset <= m.End() {
			return true
		}
	}
	return false
}

// hasError reports whether the submission failed in a way that leaves it
// nothing to summarize; a notation error still reads, so it is not one.
func hasError(diags []passes.Diagnostic) bool {
	for _, d := range diags {
		if d.Blocking() {
			return true
		}
	}
	return false
}

// within narrows the result to one span of the submission — one file of a load
// of several — so what is reported as its own is scoped to that text alone. A
// member is the file's when it begins there: the transcript's last member runs
// on over the files' text masked out after it.
func (r Result) within(span source.Span) Result {
	r.own = []source.Span{span}
	members := make([]Member, 0, len(r.Members))
	for _, m := range r.Members {
		if m.Offset >= span.Offset && m.Offset < span.End() {
			members = append(members, m)
		}
	}
	r.Members = members
	return r
}

// ownMembers returns the top-level members this submission contributed, so a
// summary does not re-announce everything typed earlier in the session.
func (r Result) ownMembers() []Member {
	out := make([]Member, 0, len(r.Members))
	for _, m := range r.Members {
		if r.holdsMine(source.Span{Offset: m.Offset, Len: m.Node.Span().Len}) {
			out = append(out, m)
		}
	}
	return out
}

// baseLine is the 1-based buffer line this submission starts on.
func (r Result) baseLine() int {
	return r.lineOf(r.Offset)
}

// wholeBuffer numbers diagnostics against the accumulated buffer, naming no file.
func wholeBuffer(int) (string, int) { return "", 1 }

// inFile reports every diagnostic against file, numbering from its first line,
// for a caller that already knows which source it is rendering.
func inFile(file string) func(int) (string, int) {
	return func(int) (string, int) { return file, 1 }
}
