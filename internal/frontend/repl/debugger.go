package repl

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
)

// DebugReport is where the session's debugging stands, for a caller driving the
// debuggers as data rather than through the prompt's text. Kind is "action" or
// "state" for the executor the report describes, "check" when the behavior was
// put to the check engine instead of stepped, and "" when no debugging session
// runs. The action fields are filled for an action, the state fields for a
// machine; Lines are what the prompt printed for the command.
type DebugReport struct {
	Kind      string
	Name      string
	Qualified string
	Performer string
	State     string
	Ended     bool
	Lines     []string

	// Action debugging.
	PausedAt    string
	Breakpoints []string
	Tokens      []TokenReport
	Results     []NamedValue

	// State machine debugging.
	CurrentState    string
	ActiveStates    []string
	Time            float64
	LastEventAt     float64
	QueuedEvents    int
	SignalsInFlight []string
	SuspendReason   string
	StateStack      []string
	StateData       []NamedValue
}

// TokenReport is one token of the debugged action: where it sits, where it came
// from and, at a join, the successions it still waits for.
type TokenReport struct {
	ID          int64
	Node        string
	ArrivedFrom string
	Awaiting    []string
}

// Debugging reports whether an action or a state machine is being debugged.
func (s *Session) Debugging() bool {
	defer s.reading()()
	return s.actionExec != nil || s.stateExec != nil
}

// DebugStatus reports the debugging session under way, with the lines %tokens
// and %current print for it; false when none runs.
func (s *Session) DebugStatus() (DebugReport, bool) {
	defer s.enter()()
	if s.actionExec == nil && s.stateExec == nil {
		return DebugReport{}, false
	}
	// The lines describe the debugger the report does, and that one only.
	var lines []string
	if s.debuggedAction() != nil {
		lines, _, _ = s.doTokens()
	} else {
		lines, _, _ = s.doCurrent()
	}
	return s.debugReport(lines), true
}

// DebugAction starts debugging an action, performed by the object performer
// names when it names one, as %action does. Under the check engine the action
// is checked instead, and the report's Kind is "check".
func (s *Session) DebugAction(name string, performer []string) (DebugReport, error) {
	defer s.enter()()
	if s.checking() {
		verdict := s.checkAction(name, performer)
		return DebugReport{Kind: "check", Name: name, Lines: verdict.Lines}, nil
	}
	lines, err := s.startAction(name, performer)
	if err != nil {
		return DebugReport{}, err
	}
	return s.debugReport(append(s.drainTrace(), lines...)), nil
}

// DebugState starts debugging a state machine, or attaches to the one an object
// exhibits, as %state does. Under the check engine the machine is checked
// instead, and the report's Kind is "check".
func (s *Session) DebugState(name string, performer []string) (DebugReport, error) {
	defer s.enter()()
	if s.checking() {
		verdict := s.checkInvocation(nil, []Behavior{{Name: name, Performer: performer}}, nil)
		return DebugReport{Kind: "check", Name: name, Lines: verdict.Lines}, nil
	}
	lines, err := s.startStateMachine(name, performer)
	if err != nil {
		s.noteIfMaterializationFailure(err)
		return DebugReport{}, err
	}
	return s.debugReport(append(s.drainTrace(), lines...)), nil
}

// DebugStep advances the debugged action one token step, or the debugged
// machine one step, as %step does. A step that failed is the error, with the
// report of where the debugger stands beside it.
func (s *Session) DebugStep() (DebugReport, error) {
	defer s.enter()()
	if s.actionExec == nil && s.stateExec == nil {
		return DebugReport{}, errors.New(s.noDebugSessionMsg())
	}
	lines, _, err := s.doStep()
	return s.reported(lines, err)
}

// DebugContinue runs the debugged action to completion, or to the first
// breakpoint a token reaches, as %continue does.
func (s *Session) DebugContinue() (DebugReport, error) {
	defer s.enter()()
	lines, _, err := s.continueAction()
	if err != nil {
		return s.debugReport(lines), err
	}
	return s.debugReport(append(s.drainTrace(), lines...)), nil
}

// DebugStop ends the debugging session under way, returning the lines %stop
// prints; an error when none runs.
func (s *Session) DebugStop() ([]string, error) {
	defer s.enter()()
	if s.actionExec == nil && s.stateExec == nil {
		return nil, errors.New(s.noDebugSessionMsg())
	}
	lines, _, err := s.doStop()
	return lines, err
}

// SetBreakpoints sets a breakpoint at each node named, clearing the ones set
// before when reset is set, and reports the breakpoints the debugged action
// then has with the lines %break prints. A node the action has none of is the
// error, naming its nodes.
func (s *Session) SetBreakpoints(nodes []string, reset bool) (DebugReport, error) {
	defer s.enter()()
	if s.actionExec == nil {
		return DebugReport{}, errors.New(s.noActionSessionMsg())
	}
	if reset {
		s.actionExec.executor.ClearBreakpoints()
	}
	var lines []string
	for _, node := range nodes {
		out, _, err := s.doBreak(node)
		lines = append(lines, out...)
		if err != nil {
			return s.debugReport(lines), err
		}
		if failed := failedLine(out); failed != nil {
			return s.debugReport(lines), failed
		}
	}
	return s.debugReport(lines), nil
}

// SendSignal sends a signal as %send does; text is what follows the command:
// the signal, its arguments in parentheses, and `to <object>` when the target
// is not the debugged behavior's. The signal a behavior would not take is the error.
func (s *Session) SendSignal(text string) ([]string, error) {
	defer s.enter()()
	return s.sendSignal(text)
}

// Advance moves the debuggers' clock by duration time units, as %advance does,
// and reports what ran with where the debuggers then stand. A failed step is the
// error, with the lines of what ran before it.
func (s *Session) Advance(duration float64) (DebugReport, error) {
	defer s.enter()()
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 0 {
		return DebugReport{}, fmt.Errorf("invalid time %v (must be a finite, non-negative number of time units)", duration)
	}
	lines, err := s.advanceBy(duration)
	if err != nil {
		return s.debugReport(lines), err
	}
	return s.debugReport(append(s.drainTrace(), lines...)), nil
}

// reported pairs a command's lines with where the debuggers stand, reporting a
// line the prompt prints as an error as one.
func (s *Session) reported(lines []string, err error) (DebugReport, error) {
	if err != nil {
		return s.debugReport(lines), err
	}
	report := s.debugReport(append(s.drainTrace(), lines...))
	return report, failedLine(lines)
}

// failedLine is the first line the prompt printed as an error, as an error; nil
// when none was.
func failedLine(lines []string) error {
	for _, line := range lines {
		if strings.HasPrefix(line, errPrefix) {
			return errors.New(strings.TrimPrefix(line, errPrefix))
		}
	}
	return nil
}

// debugReport describes the debugging session under way. With both an action
// and a machine debugged it describes the one still running, as a completed
// run is no longer what a caller steps, and the action when both run, as %step
// prefers it.
func (s *Session) debugReport(lines []string) DebugReport {
	report := DebugReport{Lines: lines}
	if lines == nil {
		report.Lines = []string{}
	}
	switch {
	case s.debuggedAction() != nil:
		a := s.actionExec
		exec := a.executor
		report.Kind, report.Name, report.Qualified, report.Performer = "action", a.name, a.fqn, a.selfFQN
		report.State, report.Ended = exec.State().String(), exec.State().Ended()
		report.PausedAt = exec.PausedAt()
		report.Breakpoints = exec.Breakpoints()
		for _, tok := range exec.Tokens() {
			t := TokenReport{ID: tok.ID, Node: nodeLabel(tok.Location)}
			if awaiting := exec.Awaiting(tok); len(awaiting) > 0 {
				t.ArrivedFrom = nodeLabel(tok.Via.Source)
				for _, edge := range awaiting {
					t.Awaiting = append(t.Awaiting, nodeLabel(edge.Source))
				}
			}
			report.Tokens = append(report.Tokens, t)
		}
		report.Results = namedValues(a.contextOf(), exec.Results())
	case s.stateExec != nil:
		st := s.stateExec
		exec := st.executor
		report.Kind, report.Name, report.Qualified, report.Performer = "state", st.name, st.fqn, st.selfFQN
		report.State, report.Ended = exec.State().String(), exec.State().Ended()
		report.CurrentState = currentStateName(exec)
		for _, state := range exec.ActiveStates() {
			name := state.Name
			if name == "" {
				name = "<anonymous>"
			}
			report.ActiveStates = append(report.ActiveStates, name)
		}
		report.Time, report.LastEventAt = exec.CurrentTime(), exec.LastEventAt()
		report.QueuedEvents = exec.EventQueue().Len()
		if signals, err := s.signalsInFlight(exec); err == nil {
			for _, msg := range signals {
				report.SignalsInFlight = append(report.SignalsInFlight, signalText(msg))
			}
		}
		report.SuspendReason = exec.SuspendReason()
		for _, node := range exec.StateStack() {
			if node.Name != "" {
				report.StateStack = append(report.StateStack, node.Name)
			}
		}
		report.StateData = namedValues(st.contextOf(), exec.StateData())
	}
	return report
}

// FeatureListing is how far Features expands an object: every nested object
// when All is set, to Depth when it is non-negative, and bounded as the prompt
// bounds a plain %features otherwise.
type FeatureListing struct {
	Depth int
	All   bool
}

// FeaturesReport is an object's feature values as %features lists them, and the
// object graph as the API's Instantiate writes it in JSON, which is nil when the
// build does not link the graph writer.
type FeaturesReport struct {
	Lines []string
	Graph []byte
}

// Features lists an object's feature values and what its behaviors are doing,
// as %features does; object is an object reference. An object the session does
// not hold is the error.
func (s *Session) Features(object string, l FeatureListing) (FeaturesReport, error) {
	defer s.enter()()
	listing := featureListing{depth: maxFeatureValueDepth, budget: maxFeatureValueLines}
	bounded := true
	switch {
	case l.All:
		listing.depth, listing.budget = math.MaxInt, math.MaxInt
		bounded = false
	case l.Depth >= 0:
		listing.depth, listing.budget = l.Depth, math.MaxInt
		bounded = false
	}
	lines, _, err := s.doFeatures(object, listing)
	if err != nil {
		return FeaturesReport{}, err
	}
	if failed := failedLine(lines); failed != nil {
		return FeaturesReport{Lines: lines}, failed
	}
	report := FeaturesReport{Lines: lines}
	if replext.Graph() == nil {
		return report, nil
	}
	graph := listing
	graph.json = true
	if bounded {
		graph.budget = maxFeatureGraphInstances()
	}
	out, _, err := s.doFeatures(object, graph)
	if err != nil {
		return report, err
	}
	report.Graph = []byte(strings.Join(out, "\n"))
	return report, nil
}

// InstanceReport is one object the session holds: the name %instantiate created
// it under, its id, its type, and how its lifetime ended when it did. An object a
// later %instantiate of its name displaced has no Name and names what it was.
type InstanceReport struct {
	Name          string
	ID            int64
	Type          string
	DisplacedFrom string
	Life          string
}

// Instances lists the objects the session holds, as %instances does, with the
// lines it prints.
func (s *Session) Instances() ([]InstanceReport, []string) {
	defer s.enter()()
	lines, _, _ := s.doInstances()
	var out []InstanceReport
	names := make([]string, 0, len(s.instances))
	for name := range s.instances {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		inst := s.instances[name]
		out = append(out, InstanceReport{Name: s.declaredName(name), ID: inst.ID, Type: typeName(inst), Life: lifeWord(s.lifeNote(inst))})
	}
	for _, u := range s.unnamed {
		if s.rtCtx == nil {
			break
		}
		if held, ok := s.rtCtx.Instance(u.obj.ID); !ok || held != u.obj {
			continue
		}
		out = append(out, InstanceReport{ID: u.obj.ID, Type: typeName(u.obj), DisplacedFrom: s.declaredName(u.fqn), Life: lifeWord(s.lifeNote(u.obj))})
	}
	return out, lines
}

// typeName is the qualified name of the declaration an object instantiates.
func typeName(inst *runtime.Instance) string {
	if inst == nil || inst.Type == nil {
		return ""
	}
	return declarationNotation(inst.Type)
}

// lifeWord is the word of a listing's life note: "destroyed", "ended" or "".
func lifeWord(note string) string {
	return strings.TrimPrefix(note, ", ")
}
