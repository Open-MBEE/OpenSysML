package repl

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// errRuntimeInit marks a runtime the session could not create at all, which the
// prompt reports as a command error rather than as a line of output.
var errRuntimeInit = errors.New("runtime init")

// NamedValue is one value a check or a run produced, formatted as the prompt
// prints it: a calculation's result, an action's output, a machine's state.
type NamedValue struct {
	Name  string
	Value string
	// OrderDependent counts the run's unresolved choice points touching the
	// value, which the prompt prints the marker of after it; 0 for a value
	// nothing order-dependent wrote.
	OrderDependent int
}

// Text is the value as the prompt prints it, marked when order-dependent.
func (v NamedValue) Text() string {
	return markOrderDependent(v.Value, v.OrderDependent)
}

// namedValues lists an executor's results in name order, so two reports of the
// same run read the same way.
func namedValues(ctx *runtime.Context, results map[string]runtime.Value) []NamedValue {
	if len(results) == 0 {
		return nil
	}
	names := make([]string, 0, len(results))
	for name := range results {
		names = append(names, name)
	}
	slices.Sort(names)
	values := make([]NamedValue, 0, len(names))
	for _, name := range names {
		values = append(values, namedValue(ctx, name, formatValue(ctx, results[name])))
	}
	return values
}

// namedValue is the value under its name, marked when the run's unresolved
// choice points touch it.
func namedValue(ctx *runtime.Context, name, text string) NamedValue {
	v := NamedValue{Name: name, Value: text}
	if ctx != nil {
		v.OrderDependent = ctx.OrderDependent(name)
	}
	return v
}

// markOrderDependent appends the order-dependence marker to a value's text.
func markOrderDependent(text string, unresolved int) string {
	if marker := runtime.OrderDependenceText(unresolved); marker != "" {
		return text + " " + marker
	}
	return text
}

// errorLines adapts a command that reports its failures as errors to the
// prompt, which prints them as output ahead of any lines the failed run left.
func errorLines(lines []string, _ []NamedValue, err error) ([]string, bool, error) {
	if err != nil {
		return append([]string{"error: " + err.Error()}, lines...), false, nil
	}
	return lines, false, nil
}

// LoadFile submits path and the files beside and below it that declare a root
// namespace it references as one submission, returning the lines `%load` prints.
// A lone "-" reads standard input; the error is a file it could not read or
// one named as the transcript is.
func (s *Session) LoadFile(path string) ([]string, error) {
	defer s.enter()()
	files, err := s.expandPaths([]string{path})
	if err != nil {
		return nil, err
	}
	read, err := s.readSources(files, loadOptions{})
	if err != nil {
		return nil, err
	}
	lines := read.notes
	if len(read.files) > 0 {
		lines = append(lines, renderResult(s.submitFiles(read.files), s.verbosity)...)
	}
	return append(lines, conversionWarnings(read.files, s.verbosity)...), nil
}

// LoadFileSummary submits the contents of path and returns only what it
// declared, without what analysis has found so far. A caller loading a model
// that spans several files reports the analysis once every file is in, a
// reference from one file to another resolving only then.
func (s *Session) LoadFileSummary(path string) ([]string, error) {
	return s.LoadFilesSummary([]string{path})
}

// LoadFilesSummary is LoadFileSummary over every path as one submission, each
// file a document of its own, indexed together and each summarized on its own;
// a read failure is a *ReadError and a file named as the transcript is a
// *ReservedNameError. Files beside and below the paths that declare a root
// namespace they reference load too.
func (s *Session) LoadFilesSummary(paths []string) ([]string, error) {
	defer s.enter()()
	paths, err := s.expandArchives(expandHomes(paths))
	if err != nil {
		return nil, err
	}
	read, err := s.readSources(paths, loadOptions{})
	if err != nil {
		return nil, err
	}
	files := read.files
	lines := read.notes
	archives := s.discoveryWarnings(paths)
	if len(files) == 0 {
		return append(lines, archives...), nil
	}
	res, byFile, whole := s.submitEach(files)
	for i, f := range files {
		// The analysis is reported once every file is in, but a file that does not
		// parse is a finding about that file alone and is reported with it.
		own := res.within(s.fileSpan(f.key()))
		lines = append(lines, renderSyntax(own, s.verbosity)...)
		lines = append(lines, byFile[i]...)
		lines = append(lines, conversionWarnings([]SourceFile{f}, s.verbosity)...)
		if i == 0 {
			lines = append(lines, whole...)
		}
		lines = append(lines, renderSummary(own.ownMembers())...)
	}
	// The files loaded for the roots the named ones left unresolved are
	// summarized as the named ones are.
	for _, path := range res.pulled {
		own := res.within(s.fileSpan(fileKeyOf(path)))
		lines = append(lines, renderSyntax(own, s.verbosity)...)
		lines = append(lines, renderSummary(own.ownMembers())...)
	}
	return append(lines, archives...), nil
}

// DiagnosticLines reports the analysis of everything submitted so far as the
// prompt prints it: the source line each finding is on, under a position naming
// the file the finding is in, at the verbosity the session was asked for.
func (s *Session) DiagnosticLines() []string {
	defer s.enter()()
	return s.diagnosticLines()
}

func (s *Session) diagnosticLines() []string {
	diags := s.diagnostics()
	if len(diags) == 0 {
		return s.libraryFindingLines()
	}
	var out []string
	start := 0
	for i, sn := range s.snippets {
		end := start + len(sn.src)
		var own []diag.Diagnostic
		for _, d := range diags {
			if d.Span.Offset < start || (d.Span.Offset > end && i != len(s.snippets)-1) {
				continue
			}
			if d.Severity != diag.SeverityError && s.verbosity <= VerbosityQuiet {
				continue
			}
			d.Span.Offset -= start
			own = append(own, d)
		}
		out = append(out, renderDiagnostics(own, sn.src, inFile(sn.origin), s.verbosity >= VerbosityDebug)...)
		start = end + 1 // the newline joined() writes between snippets
	}
	return append(out, s.libraryFindingLines()...)
}

// libraryFindingLines renders the workspace findings on library documents — a
// library tree the library path holds two copies of — which no submission's
// text locates, under the library file each is reported in.
func (s *Session) libraryFindingLines() []string {
	if s.verbosity <= VerbosityQuiet {
		return nil
	}
	var out []string
	for _, f := range s.ws.WorkspaceFindings() {
		if !f.Library {
			continue
		}
		src := ""
		if doc := s.ws.LibraryDocument(f.Document); doc != nil {
			src = string(doc.Content)
		}
		out = append(out, renderDiagnostics([]diag.Diagnostic{f.Diagnostic}, src, inFile(f.Document), s.verbosity >= VerbosityDebug)...)
	}
	return out
}

// Diagnostics reports the analysis of everything submitted so far, including the
// syntax errors of a submission masked out of the buffer for not closing its own
// text: a load whose file does not parse says why, and HasErrors is true, which is
// what a non-interactive run exits on.
func (s *Session) Diagnostics() []diag.Diagnostic {
	defer s.enter()()
	return s.diagnostics()
}

// HasErrors reports whether the session found the model wrong: something analysis
// found, or a feature value a command could not materialize. It is what a non-interactive
// run exits on.
func (s *Session) HasErrors() bool {
	defer s.enter()()
	return s.hasAnalysisErrors() || len(s.materializeFailures) > 0
}

// hasAnalysisErrors reports whether analysis found something that stops the model from
// running. Notation stops it only when asked strictly, which rejects the file outright.
func (s *Session) hasAnalysisErrors() bool {
	strict := s.ws.ConformanceMode().IsStrict()
	for _, d := range s.allDiagnostics() {
		if d.Blocking() || (strict && d.Severity == diag.SeverityError) {
			return true
		}
	}
	return false
}

// MaterializationFailures reports the feature values the session's commands could not
// materialize, in the order they were reported: a command that rendered one
// answered nothing about that feature value.
func (s *Session) MaterializationFailures() []error {
	defer s.enter()()
	return slices.Clone(s.materializeFailures)
}

// noteIfMaterializationFailure records an error a command reported when it is a
// feature value that could not be materialized, so which command surfaced it — a feature value
// listing, an evaluation, a pinned one — does not decide whether it is recorded.
// Callers hold s.mu.
func (s *Session) noteIfMaterializationFailure(err error) {
	if errors.Is(err, runtime.ErrFeatureValueMaterialization) {
		s.noteMaterializationFailure(err)
	}
}

// noteMaterializationFailure records feature values a command could not materialize. It is
// a record of what the session answered, so it stands once the object is gone.
// Callers hold s.mu.
func (s *Session) noteMaterializationFailure(errs ...error) {
	for _, err := range errs {
		if err != nil {
			s.materializeFailures = append(s.materializeFailures, err)
		}
	}
}

// Diagnostic is one finding about the session's model, located in it, for a
// caller reporting analysis as data rather than as the prompt's text.
type Diagnostic struct {
	Severity string
	Message  string
	// File is the loaded file the finding is in, empty for typed input, and Line
	// and Column place it within that file rather than within the session buffer.
	File   string
	Line   int
	Column int
	// UTF16Column is Column counted in UTF-16 code units, as SARIF counts.
	UTF16Column int
	// Pass names what produced the finding, and Code what it found.
	Pass string
	Code string
	// Fixes are the edits that resolve the finding, when analysis knows them.
	Fixes []Fix
}

// Fix is one way to resolve a diagnostic: a set of edits to the file it is in.
type Fix struct {
	Title     string
	Preferred bool
	Edits     []Edit
}

// Edit replaces the text from Line:Column up to EndLine:EndColumn (exclusive,
// 1-based, within the diagnostic's file) with NewText; an insertion has an empty span.
type Edit struct {
	Line, Column       int
	EndLine, EndColumn int
	// UTF16Column and UTF16EndColumn are the columns in UTF-16 code units.
	UTF16Column, UTF16EndColumn int
	NewText                     string
}

// LocatedDiagnostics reports the analysis of everything submitted so far, each
// finding placed in the submission it is about: the file it was loaded from, at
// the line and column that file has it on.
func (s *Session) LocatedDiagnostics() []Diagnostic {
	defer s.enter()()
	return append(locate(s.bufferParts(), s.diagnostics()), s.libraryFindings()...)
}

// libraryFindings places the workspace findings on library documents — which
// no submission's text holds — in the library file each is reported in.
func (s *Session) libraryFindings() []Diagnostic {
	var out []Diagnostic
	for _, f := range s.ws.WorkspaceFindings() {
		if !f.Library {
			continue
		}
		var content []byte
		if doc := s.ws.LibraryDocument(f.Document); doc != nil {
			content = doc.Content
		}
		p := source.New(docName, content).Lines().PosAt(f.Diagnostic.Span.Offset)
		out = append(out, Diagnostic{
			Severity:    f.Diagnostic.Severity.String(),
			Message:     f.Diagnostic.Message,
			File:        f.Document,
			Line:        p.Line,
			Column:      p.Col,
			UTF16Column: utf16Column(content, f.Diagnostic.Span.Offset, p.Col),
			Pass:        f.Diagnostic.Source,
			Code:        f.Diagnostic.Code,
		})
	}
	return out
}

// Locate places diags, a Result's diagnostics, in the submissions they are about
// as LocatedDiagnostics does — in the buffer they were reported over, if res left it.
func (s *Session) Locate(res Result, diags []diag.Diagnostic) []Diagnostic {
	defer s.enter()()
	parts := res.buffer
	if parts == nil {
		parts = s.bufferParts()
	}
	return locate(parts, diags)
}

// bufferPart is one submission as the joined buffer holds it: its text, the
// file it was loaded from (empty when typed) and the offset it starts at.
type bufferPart struct {
	origin, src string
	start       int
}

// bufferParts lays the session's submissions out as its buffer joins them.
func (s *Session) bufferParts() []bufferPart {
	parts := make([]bufferPart, 0, len(s.snippets))
	start := 0
	for _, sn := range s.snippets {
		parts = append(parts, bufferPart{origin: sn.origin, src: sn.src, start: start})
		start += len(sn.src) + 1 // the newline joined() writes between snippets
	}
	return parts
}

// partAt returns the submission a buffer offset falls in, as snippetAt does.
func partAt(parts []bufferPart, offset int) bufferPart {
	for i, part := range parts {
		if offset <= part.start+len(part.src) || i == len(parts)-1 {
			return part
		}
	}
	return bufferPart{}
}

func locate(parts []bufferPart, diags []diag.Diagnostic) []Diagnostic {
	if len(diags) == 0 {
		return nil
	}
	out := make([]Diagnostic, 0, len(diags))
	type indexed struct {
		src   []byte
		lines *source.LineIndex
	}
	indexes := make(map[int]indexed, len(parts))
	for _, d := range diags {
		part := partAt(parts, d.Span.Offset)
		start := part.start
		idx, ok := indexes[start]
		if !ok {
			idx.src = []byte(part.src)
			idx.lines = source.New(docName, idx.src).Lines()
			indexes[start] = idx
		}
		lines := idx.lines
		p := lines.PosAt(d.Span.Offset - start)
		out = append(out, Diagnostic{
			Severity:    d.Severity.String(),
			Message:     d.Message,
			File:        part.origin,
			Line:        p.Line,
			Column:      p.Col,
			UTF16Column: utf16Column(idx.src, d.Span.Offset-start, p.Col),
			Pass:        d.Source,
			Code:        d.Code,
			Fixes:       locatedFixes(d.Fixes, idx.src, lines, start),
		})
	}
	return out
}

// locatedFixes places a diagnostic's fixes in its file by line and column, the
// edits rendered against the file so an own-line insertion carries its indent.
func locatedFixes(fixes []diag.Fix, content []byte, lines *source.LineIndex, start int) []Fix {
	if len(fixes) == 0 {
		return nil
	}
	out := make([]Fix, 0, len(fixes))
	for _, f := range fixes {
		fix := Fix{Title: f.Title, Preferred: f.Preferred}
		for _, e := range f.Edits {
			e.Span.Offset -= start
			span, text := e.Render(content)
			from, to := lines.PosAt(span.Offset), lines.PosAt(span.End())
			fix.Edits = append(fix.Edits, Edit{
				Line: from.Line, Column: from.Col, EndLine: to.Line, EndColumn: to.Col, NewText: text,
				UTF16Column:    utf16Column(content, span.Offset, from.Col),
				UTF16EndColumn: utf16Column(content, span.End(), to.Col),
			})
		}
		out = append(out, fix)
	}
	return out
}

// utf16Column is the column of offset, whose byte column is col, counted in
// UTF-16 code units as SARIF and the editors count; 0 stays the whole line.
func utf16Column(content []byte, offset, col int) int {
	lineStart := offset - (col - 1)
	if col < 1 || lineStart < 0 || offset > len(content) {
		return col
	}
	return utf16Units(content[lineStart:offset]) + 1
}

// utf16Units counts the UTF-16 code units of b without encoding it; an invalid
// byte counts as the one unit of the replacement character, as utf16.Encode does.
func utf16Units(b []byte) int {
	n := 0
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		b = b[size:]
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// Suppress keeps the diagnostics the predicate accepts out of everything the
// session reports about its model: the lines, the located findings and whether
// it has errors. Suppressed counts how many the last report left out.
func (s *Session) Suppress(accept func(file, code, message string) bool) {
	defer s.enter()()
	s.suppress = accept
}

// Suppressed is how many diagnostics the predicate given to Suppress kept out
// of the session's last report.
func (s *Session) Suppressed() int {
	defer s.enter()()
	s.diagnostics()
	return s.suppressed
}

// snippetAt returns the submission a session-buffer offset falls in and the
// offset that submission starts at, joined() being the snippets with a newline
// between them.
func (s *Session) snippetAt(offset int) (snippet, int) {
	start := 0
	for i, sn := range s.snippets {
		end := start + len(sn.src)
		if offset <= end || i == len(s.snippets)-1 {
			return sn, start
		}
		start = end + 1 // the newline joined() writes between snippets
	}
	return snippet{}, 0
}

// EvalExpr evaluates an expression and returns the lines `%eval` prints, with an
// error for one that could not be evaluated.
func (s *Session) EvalExpr(expr string) ([]string, error) {
	defer s.enter()()
	lines, err := s.evalExpr(expr)
	if err != nil {
		return nil, err
	}
	return append(s.drainTrace(), lines...), nil
}

// EvalBare evaluates a prompt line read as an expression, recording a failed
// materialization so a piped run's exit status reports it, as %eval does.
func (s *Session) EvalBare(expr string) ([]string, error) {
	defer s.enter()()
	lines, err := s.evalExpr(expr)
	if err != nil {
		s.noteIfMaterializationFailure(err)
		return nil, err
	}
	return append(s.drainTrace(), lines...), nil
}

// RunCalc invokes a calculation and returns what it computed. invocation is
// what `%calc` takes: a name, optionally followed by its arguments or carrying
// them as `Fall(3, 4)`.
func (s *Session) RunCalc(invocation string) Verdict {
	defer s.enter()()
	if _, explores := s.exploring(); explores {
		return s.exploreCalc(invocation)
	}
	name, argText := splitCalcArgs(invocation)
	return s.withTrace(s.calcVerdict(name, argText))
}

// RunAnalysis runs an analysis case outside the prompt and returns what it
// computed and decided. invocation is what `%analysis` takes: a name, optionally
// carrying arguments as `Case(3.0, limit = 4.0)`, then the object that is its
// subject. Its status is the worst its objective and assertions decided.
func (s *Session) RunAnalysis(invocation string) Verdict {
	defer s.enter()()
	inv, err := splitAnalysisArgs(invocation)
	if err != nil {
		return s.withTrace(unresolvedVerdict(invocation, err.Error()))
	}
	if _, explores := s.exploring(); explores {
		return s.exploreAnalysis(inv)
	}
	return s.withTrace(s.analysisVerdict(inv))
}

// RunAction runs an action to completion outside the prompt, on the object
// performer names when it names one. An action that could not be run, or that
// stopped short of completing, is unresolved: it produced no outputs to judge.
func (s *Session) RunAction(name string, performer ...string) Verdict {
	return s.RunActionWithInputs(name, nil, performer...)
}

// RunActionWithInputs is RunAction with the action's input parameters bound
// before it starts, from arguments written positionally, in parameter order, or
// as `<parameter>=<expression>`, each evaluated where the prompt evaluates one.
// Inputs bind a run only; under check or explore, which search every schedule
// from the action's own start, the verdict is unresolved and says so.
func (s *Session) RunActionWithInputs(name string, inputs []string, performer ...string) Verdict {
	defer s.enter()()
	parsed, err := parseInvokeArgumentList(inputs)
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	given := len(parsed.positional) + len(parsed.named)
	if s.checking() {
		if given > 0 {
			return s.withTrace(unresolvedVerdict(name, "inputs are not bound under check: a check searches the action's schedules from its own start, so run it without inputs or under run mode"))
		}
		return s.checkAction(name, performer)
	}
	if _, explores := s.exploring(); explores {
		if given > 0 {
			return s.withTrace(unresolvedVerdict(name, "inputs are not bound under explore: an exploration searches the action's schedules from its own start, so run it without inputs or under run mode"))
		}
		return s.exploreAction(name, performer)
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, fmt.Errorf("%w: %w", errRuntimeInit, err).Error()))
	}
	done, plan, err := evaluate(s.dispatched(), name, ctx, func(*runtime.Context) (performed, error) {
		started, err := s.startActionWithInputs(name, performer, parsed)
		if err != nil {
			return performed{}, err
		}
		lines, values, err := s.continueAction()
		return performed{lines: append(started, lines...), values: values}, err
	}, func(_ performed, err error) analysis.Answer {
		if err != nil {
			return analysis.Answer{Err: err}
		}
		exec := s.actionExec.executor
		return behaviorAnswer(exec.State().Ended(), "stopped at "+exec.State().String(), exec.Results(), nil)
	})
	if err != nil {
		return s.withTrace(standing(unresolvedVerdict(name, err.Error()), plan))
	}
	lines, values := done.lines, done.values
	if state := s.actionExec.executor.State(); !state.Ended() {
		lines = append(lines, fmt.Sprintf("error: action %s stopped at %s without completing", name, state))
		return s.withTrace(standing(Verdict{Subject: name, Status: VerdictUnresolved, Lines: lines, Values: values}, plan))
	}
	return s.withTrace(standing(Verdict{Subject: name, Status: VerdictHolds, Lines: lines, Values: values}, plan))
}

// RunOccurrence performs an occurrence def as the interaction its body states,
// which is `%occurrence`: each message is transferred from the lifeline holding
// the event it leaves to the one holding the event it reaches, in the order the
// events admit. The values are the transfers performed, in order.
func (s *Session) RunOccurrence(name string) Verdict {
	defer s.enter()()
	return s.runOccurrence(name)
}

// runOccurrence is RunOccurrence inside a command already holding the session,
// as `%occurrence` is.
func (s *Session) runOccurrence(name string) Verdict {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, fmt.Errorf("%w: %w", errRuntimeInit, err).Error()))
	}
	sym, _, err := s.lookupSymbolOfKinds(name, symbols.SymbolOccurrenceDef, symbols.SymbolOccurrenceUsage)
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	run, err := ctx.ExecuteOccurrence(sym)
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	values := make([]NamedValue, 0, len(run.Transfers))
	lines := make([]string, 0, len(run.Transfers))
	for i, tr := range run.Transfers {
		text := fmt.Sprintf("%s -> %s", tr.FromEvent, tr.ToEvent)
		values = append(values, NamedValue{Name: fmt.Sprintf("%d %s", i+1, tr.Name), Value: text})
		lines = append(lines, fmt.Sprintf("message %s: %s", tr.Name, text))
	}
	return Verdict{Subject: name, Status: VerdictHolds, Lines: lines, Values: values}
}

// RunStateMachine starts a state machine outside the prompt, taking only its
// initial transition, which is `%state` alone. The values are the configuration
// the machine settled in. Under the check engine every schedule of that initial
// transition is searched instead.
func (s *Session) RunStateMachine(name string, performer ...string) Verdict {
	defer s.enter()()
	if s.checking() {
		return s.checkInvocation(nil, []Behavior{{Name: name, Performer: performer}}, nil)
	}
	return s.runStateMachine(name, nil, performer)
}

// RunStateMachineFor starts a state machine and advances it by duration time
// units, which is `%state` followed by `%advance`. A duration of 0 is a run to
// the current time, dispatching the events already due, as `%advance 0` is.
// Under the check engine every schedule up to that instant is searched.
func (s *Session) RunStateMachineFor(name string, duration float64, performer ...string) Verdict {
	defer s.enter()()
	if s.checking() {
		return s.checkInvocation(nil, []Behavior{{Name: name, Performer: performer}}, &duration)
	}
	return s.runStateMachine(name, &duration, performer)
}

// Behavior names a behavior to run and, after it, the object performing it, as
// `-action`/`-state` take them: `Drive rover1`.
type Behavior struct {
	Name      string
	Performer []string
}

// RunFor starts the behaviors named, advances their shared clock by duration once
// and returns one verdict per behavior (an action holds when it completed in time).
// Under the check engine the behaviors are one invocation, every schedule of which
// up to that instant is searched: one verdict for them all.
func (s *Session) RunFor(actions, states []Behavior, duration float64) []Verdict {
	defer s.enter()()
	if s.checking() {
		return s.checkInvocations(actions, states, &duration)
	}
	if _, explores := s.exploring(); explores {
		return s.exploreRunFor(actions, states, duration)
	}

	type run struct {
		at     int
		name   string
		lines  []string
		action *actionSession
		state  *stateSession
	}
	runs := make([]*run, 0, len(actions)+len(states))
	verdicts := make([]Verdict, len(actions)+len(states))
	var lastAction *actionSession
	for i, b := range actions {
		lines, err := s.startAction(b.Name, b.Performer)
		if err != nil {
			verdicts[i] = unresolvedVerdict(b.Name, err.Error())
			continue
		}
		lastAction = s.actionExec
		runs = append(runs, &run{at: i, name: b.Name, lines: lines, action: s.actionExec})
		s.actionExec = nil
	}
	var lastState *stateSession
	for i, b := range states {
		lines, err := s.startStateMachine(b.Name, b.Performer)
		if err != nil {
			verdicts[len(actions)+i] = unresolvedVerdict(b.Name, err.Error())
			continue
		}
		lastState = s.stateExec
		runs = append(runs, &run{at: len(actions) + i, name: b.Name, lines: lines, state: s.stateExec})
		s.stateExec = nil
	}
	if lastAction != nil {
		s.actionExec = lastAction
	}
	if lastState != nil {
		s.stateExec = lastState
	}
	defer func() {
		for _, r := range runs {
			if r.action != nil && r.action != lastAction {
				r.action.release()
			}
			if r.state != nil && r.state != lastState {
				r.state.release()
			}
		}
	}()

	var contexts []*runtime.Context
	names := make([]string, 0, len(runs))
	for _, r := range runs {
		contexts = append(contexts, r.action.contextOf(), r.state.contextOf())
		names = append(names, r.name)
	}
	contexts = distinctContexts(contexts...)
	var (
		moved  advanceOutcome
		failed []string
		plan   *analysis.Plan
		err    error
	)
	if len(contexts) > 0 {
		moved, failed, plan, err = s.advanceFor(strings.Join(names, ", "), contexts, duration)
	}

	// The drain is one operation over every behavior, so what it did is reported
	// once, with the first behavior that ran; each then reports where it stands.
	for i, r := range runs {
		v := Verdict{Subject: r.name, Status: VerdictHolds, Lines: r.lines}
		if err != nil {
			v.Status = VerdictUnresolved
			v.Lines = append(v.Lines, failed...)
			v.Lines = append(v.Lines, "error: "+err.Error())
			verdicts[r.at] = standing(v, plan)
			continue
		}
		if i == 0 {
			v.Lines = append(v.Lines, advancedHeader(moved.report))
		}
		var outcome []string
		switch {
		case r.state != nil:
			exec := r.state.executor
			v.Lines = append(v.Lines, stateStatusLines(exec)...)
			if exec.State().Ended() {
				outcome = []string{stateEndedText(exec)}
			}
			v.Values = []NamedValue{
				{Name: "state", Value: currentStateName(exec)},
				{Name: "time", Value: semantics.FormatReal(exec.CurrentTime())},
			}
			v.Values = append(v.Values, namedValues(r.state.contextOf(), exec.StateData())...)
		case r.action != nil:
			exec := r.action.executor
			v.Lines = append(v.Lines, actionStatusLines(exec)...)
			if exec.State().Ended() {
				outcome = append([]string{actionEndedText(exec)}, renderResults(r.action.contextOf(), exec.Results())...)
				v.Values = namedValues(r.action.contextOf(), exec.Results())
			} else {
				v.Status = VerdictUnresolved
				outcome = []string{fmt.Sprintf("error: action %s stopped at %s at simulation time %s without completing",
					r.name, exec.State(), semantics.FormatReal(r.action.rtCtx.Clock().Now()))}
				outcome = append(outcome, tokenWaitLines(exec)...)
				if next, ok := exec.NextWait(); ok {
					outcome = append(outcome, fmt.Sprintf("  The clock must reach t=%s for it to go on; run it with -advance %s or more",
						semantics.FormatReal(next), semantics.FormatReal(next-moved.report.From)))
				}
			}
		}
		if i == 0 {
			v.Lines = append(v.Lines, s.advanceReportLines(moved, contexts)...)
		}
		v.Lines = append(v.Lines, outcome...)
		verdicts[r.at] = standing(v, plan)
	}
	if len(verdicts) > 0 {
		verdicts[0] = s.withTrace(verdicts[0])
	}
	return verdicts
}

func (s *Session) runStateMachine(name string, duration *float64, performer []string) Verdict {
	if _, explores := s.exploring(); explores {
		return s.exploreStateMachine(name, duration, performer)
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, fmt.Errorf("%w: %w", errRuntimeInit, err).Error()))
	}
	lines, plan, err := evaluate(s.dispatched(), name, ctx, func(*runtime.Context) ([]string, error) {
		lines, err := s.startStateMachine(name, performer)
		if err != nil {
			return nil, err
		}
		if duration != nil {
			advanced, err := s.advanceBy(*duration)
			if err != nil {
				return nil, err
			}
			lines = append(lines, advanced...)
		}
		return lines, nil
	}, func(_ []string, err error) analysis.Answer {
		if err != nil {
			return analysis.Answer{Err: err}
		}
		return behaviorAnswer(true, "", s.stateExec.executor.StateData(), nil)
	})
	if err != nil {
		return s.withTrace(standing(unresolvedVerdict(name, err.Error()), plan))
	}
	exec := s.stateExec.executor
	values := []NamedValue{
		{Name: "state", Value: currentStateName(exec)},
		{Name: "time", Value: semantics.FormatReal(exec.CurrentTime())},
	}
	values = append(values, namedValues(s.stateExec.contextOf(), exec.StateData())...)
	return s.withTrace(standing(Verdict{Subject: name, Status: VerdictHolds, Lines: lines, Values: values}, plan))
}

// expandArchives replaces each project interchange archive among paths with
// its model files, as naming it on the command line does; every other path
// is kept as given, so a file read failure stays the caller's to report. A
// path reached twice — an archive and one of its members, two archives using
// the same project — is kept once, in first position.
func (s *Session) expandArchives(paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	add := func(path string) {
		key := path
		if !project.IsStdin(path) {
			if abs, err := filepath.Abs(path); err == nil {
				key = abs
			}
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, path)
		}
	}
	for _, path := range paths {
		if !project.IsArchiveFile(path) {
			add(path)
			continue
		}
		files, err := s.discovery().Expand([]string{path})
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			add(file)
		}
	}
	return out, nil
}
