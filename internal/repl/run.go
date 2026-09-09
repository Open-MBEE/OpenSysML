package repl

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/project"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// errRuntimeInit marks a runtime the session could not create at all, which the
// prompt reports as a command error rather than as a line of output.
var errRuntimeInit = errors.New("runtime init")

// NamedValue is one value a check or a run produced, formatted as the prompt
// prints it: a calculation's result, an action's output, a machine's state.
type NamedValue struct {
	Name  string
	Value string
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
		values = append(values, NamedValue{Name: name, Value: formatValue(ctx, results[name])})
	}
	return values
}

// errorLines adapts a command that reports its failures as errors to the
// prompt, which prints them as output ahead of any lines the failed run left.
func errorLines(lines []string, _ []NamedValue, err error) ([]string, bool, error) {
	if err != nil {
		return append([]string{"error: " + err.Error()}, lines...), false, nil
	}
	return lines, false, nil
}

// LoadFile submits the contents of path to the session and returns the lines
// `%load` prints. A lone "-" reads standard input. The error is the file it
// could not read; a model that read but did not analyse cleanly is reported by
// Diagnostics.
func (s *Session) LoadFile(path string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name, data, err := project.ReadFile(expandHome(path))
	if err != nil {
		return nil, readError(name, err)
	}
	return renderResult(s.submit(name, string(data)), s.verbosity), nil
}

// LoadFileSummary submits the contents of path and returns only what it
// declared, without what analysis has found so far. A caller loading a model
// that spans several files reports the analysis once every file is in, a
// reference from one file to another resolving only then.
func (s *Session) LoadFileSummary(path string) ([]string, error) {
	return s.LoadFilesSummary([]string{path})
}

// LoadFilesSummary is LoadFileSummary over every path as one submission, indexed and
// analyzed once, each file still summarized on its own; a read failure is a *ReadError.
func (s *Session) LoadFilesSummary(paths []string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files := make([]SourceFile, 0, len(paths))
	for _, path := range paths {
		name, data, err := project.ReadFile(expandHome(path))
		if err != nil {
			return nil, readError(name, err)
		}
		files = append(files, SourceFile{Name: name, Text: string(data)})
	}
	res, byFile, whole := s.submitEach(files)
	var lines []string
	for i, f := range files {
		// The analysis is reported once every file is in, but a file that does not
		// parse is a finding about that file alone and is reported with it.
		own := res.within(s.fileSpan(f.Name))
		lines = append(lines, renderSyntax(own, s.verbosity)...)
		lines = append(lines, byFile[i]...)
		if i == 0 {
			lines = append(lines, whole...)
		}
		lines = append(lines, renderSummary(own.ownMembers())...)
	}
	return lines, nil
}

// DiagnosticLines reports the analysis of everything submitted so far as the
// prompt prints it: the source line each finding is on, under a position naming
// the file the finding is in, at the verbosity the session was asked for.
func (s *Session) DiagnosticLines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.diagnosticLines()
}

func (s *Session) diagnosticLines() []string {
	diags := s.diagnostics()
	if len(diags) == 0 {
		return nil
	}
	var out []string
	start := 0
	for i, sn := range s.snippets {
		end := start + len(sn.src)
		var own []passes.Diagnostic
		for _, d := range diags {
			if d.Span.Offset < start || (d.Span.Offset > end && i != len(s.snippets)-1) {
				continue
			}
			if d.Severity != passes.SeverityError && s.verbosity <= VerbosityQuiet {
				continue
			}
			d.Span.Offset -= start
			own = append(own, d)
		}
		out = append(out, renderDiagnostics(own, sn.src, inFile(sn.origin), s.verbosity >= VerbosityDebug)...)
		start = end + 1 // the newline joined() writes between snippets
	}
	return out
}

// Diagnostics reports the analysis of everything submitted so far, including the
// syntax errors of a submission masked out of the buffer for not closing its own
// text: a load whose file does not parse says why, and HasErrors is true, which is
// what a non-interactive run exits on.
func (s *Session) Diagnostics() []passes.Diagnostic {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.diagnostics()
}

// HasErrors reports whether the session found the model wrong: something analysis
// found, or a feature value a command could not materialize. It is what a non-interactive
// run exits on.
func (s *Session) HasErrors() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasAnalysisErrors() || len(s.materializeFailures) > 0
}

// hasAnalysisErrors reports whether analysis found something that stops the model from
// running. Notation stops it only when asked strictly, which rejects the file outright.
func (s *Session) hasAnalysisErrors() bool {
	strict := s.ws.ConformanceMode().IsStrict()
	for _, d := range s.diagnostics() {
		if d.Blocking() || (strict && d.Severity == passes.SeverityError) {
			return true
		}
	}
	return false
}

// MaterializationFailures reports the feature values the session's commands could not
// materialize, in the order they were reported: a command that rendered one
// answered nothing about that feature value.
func (s *Session) MaterializationFailures() []error {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	// Pass names what produced the finding, and Code what it found.
	Pass string
	Code string
}

// LocatedDiagnostics reports the analysis of everything submitted so far, each
// finding placed in the submission it is about: the file it was loaded from, at
// the line and column that file has it on.
func (s *Session) LocatedDiagnostics() []Diagnostic {
	s.mu.Lock()
	defer s.mu.Unlock()
	diags := s.diagnostics()
	if len(diags) == 0 {
		return nil
	}
	out := make([]Diagnostic, 0, len(diags))
	indexes := make(map[int]*source.LineIndex, len(s.snippets))
	for _, d := range diags {
		sn, start := s.snippetAt(d.Span.Offset)
		lines, ok := indexes[start]
		if !ok {
			lines = source.New(docName, []byte(sn.src)).Lines()
			indexes[start] = lines
		}
		p := lines.PosAt(d.Span.Offset - start)
		out = append(out, Diagnostic{
			Severity: d.Severity.String(),
			Message:  d.Message,
			File:     sn.origin,
			Line:     p.Line,
			Column:   p.Col,
			Pass:     d.Source,
			Code:     d.Code,
		})
	}
	return out
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
	s.mu.Lock()
	defer s.mu.Unlock()
	lines, err := s.evalExpr(expr)
	if err != nil {
		return nil, err
	}
	return append(s.drainTrace(), lines...), nil
}

// EvalBare evaluates a prompt line read as an expression, recording a failed
// materialization so a piped run's exit status reports it, as %eval does.
func (s *Session) EvalBare(expr string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	name, argText := splitCalcArgs(invocation)
	lines, values, err := s.evalCalc(name, argText)
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	return s.withTrace(Verdict{Subject: name, Status: VerdictHolds, Lines: lines, Values: values})
}

// RunAnalysis runs an analysis case outside the prompt and returns what it
// computed and decided. invocation is what `%analysis` takes: a name, optionally
// carrying arguments as `Case(3.0, limit = 4.0)`, then the object that is its
// subject. Its status is the worst its objective and assertions decided.
func (s *Session) RunAnalysis(invocation string) Verdict {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, err := splitAnalysisArgs(invocation)
	if err != nil {
		return s.withTrace(unresolvedVerdict(invocation, err.Error()))
	}
	return s.withTrace(s.analysisVerdict(inv))
}

// RunAction runs an action to completion outside the prompt, on the object
// performer names when it names one. An action that could not be run, or that
// stopped short of completing, is unresolved: it produced no outputs to judge.
func (s *Session) RunAction(name string, performer ...string) Verdict {
	s.mu.Lock()
	defer s.mu.Unlock()
	started, err := s.startAction(name, performer)
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	lines, values, err := s.continueAction()
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	lines = append(started, lines...)
	if state := s.actionExec.executor.State(); state != runtime.StateCompleted {
		lines = append(lines, fmt.Sprintf("error: action %s stopped at %s without completing", name, state))
		return s.withTrace(Verdict{Subject: name, Status: VerdictUnresolved, Lines: lines, Values: values})
	}
	return s.withTrace(Verdict{Subject: name, Status: VerdictHolds, Lines: lines, Values: values})
}

// RunStateMachine starts a state machine outside the prompt, taking only its
// initial transition, which is `%state` alone. The values are the configuration
// the machine settled in.
func (s *Session) RunStateMachine(name string, performer ...string) Verdict {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runStateMachine(name, nil, performer)
}

// RunStateMachineFor starts a state machine and advances it by duration time
// units, which is `%state` followed by `%advance`. A duration of 0 is a run to
// the current time, dispatching the events already due, as `%advance 0` is.
func (s *Session) RunStateMachineFor(name string, duration float64, performer ...string) Verdict {
	s.mu.Lock()
	defer s.mu.Unlock()
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
func (s *Session) RunFor(actions, states []Behavior, duration float64) []Verdict {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	for _, r := range runs {
		contexts = append(contexts, r.action.contextOf(), r.state.contextOf())
	}
	contexts = distinctContexts(contexts...)
	moved, failed, err := s.advanceContexts(contexts, duration)

	// The drain is one operation over every behavior, so what it did is reported
	// once, with the first behavior that ran; each then reports where it stands.
	for i, r := range runs {
		v := Verdict{Subject: r.name, Status: VerdictHolds, Lines: r.lines}
		if err != nil {
			v.Status = VerdictUnresolved
			v.Lines = append(v.Lines, failed...)
			v.Lines = append(v.Lines, "error: "+err.Error())
			verdicts[r.at] = v
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
			if exec.State() == runtime.StateCompleted {
				outcome = []string{stateCompletedText}
			}
			v.Values = []NamedValue{
				{Name: "state", Value: currentStateName(exec)},
				{Name: "time", Value: semantics.FormatReal(exec.CurrentTime())},
			}
			v.Values = append(v.Values, namedValues(r.state.contextOf(), exec.StateData())...)
		case r.action != nil:
			exec := r.action.executor
			v.Lines = append(v.Lines, actionStatusLines(exec)...)
			if exec.State() == runtime.StateCompleted {
				outcome = append([]string{"✓ Action completed"}, renderResults(r.action.contextOf(), exec.Results())...)
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
		verdicts[r.at] = v
	}
	if len(verdicts) > 0 {
		verdicts[0] = s.withTrace(verdicts[0])
	}
	return verdicts
}

func (s *Session) runStateMachine(name string, duration *float64, performer []string) Verdict {
	started, err := s.startStateMachine(name, performer)
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	lines := started
	if duration != nil {
		advanced, err := s.advanceBy(*duration)
		if err != nil {
			return s.withTrace(unresolvedVerdict(name, err.Error()))
		}
		lines = append(lines, advanced...)
	}
	exec := s.stateExec.executor
	values := []NamedValue{
		{Name: "state", Value: currentStateName(exec)},
		{Name: "time", Value: semantics.FormatReal(exec.CurrentTime())},
	}
	values = append(values, namedValues(s.stateExec.contextOf(), exec.StateData())...)
	return s.withTrace(Verdict{Subject: name, Status: VerdictHolds, Lines: lines, Values: values})
}
