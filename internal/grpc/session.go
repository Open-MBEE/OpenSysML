package grpc

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/objref"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast/astcodec"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Session is one retained run of a cached model: a runtime context of its own,
// with its clock, its scheduling policy and the objects it instantiated, that
// successive calls read and move along. It is the engine behind the public
// client's in-process session and exists only there; it has no RPC.
type Session struct {
	svc     *Service
	cached  *CachedModel
	worker  *analysis.Worker
	release func()
	rt      *runtime.Context
	mu      sync.Mutex
	closed  bool
}

// SessionFailure is a session call the model answered with a failure — a name
// it does not declare, a run that failed — as distinct from a status the
// session refuses the call with.
type SessionFailure struct {
	Message     string
	Diagnostics []*pb.Diagnostic
}

func (f *SessionFailure) Error() string { return f.Message }

func sessionFailuref(format string, args ...any) *SessionFailure {
	return &SessionFailure{Message: fmt.Sprintf(format, args...)}
}

// runFailure classifies a runtime error: one at the session's object bound is
// the RESOURCE_EXHAUSTED status; any other is the model's failure, so described.
func (ss *Session) runFailure(err error, format string, args ...any) error {
	if errors.Is(err, runtime.ErrInstanceLimitExceeded) {
		return statusErrorf(connect.CodeResourceExhausted,
			"the session holds %d objects, and one more would pass the %d that %s allows; raise it to hold more (the objects are released when the session closes)",
			ss.rt.InstanceCount(), ss.rt.MaxInstances(), HeldObjectsEnvVar)
	}
	return sessionFailuref(format, append(args, err)...)
}

// SessionTransition is one transition of a state machine as a session reports
// it: its ends and trigger by name, no node of the graph it was lowered to.
type SessionTransition struct {
	Name    string
	Source  string
	Target  string
	Trigger string
	Signal  string
	Event   string
	Guarded bool
}

// Trigger kinds a SessionTransition reports.
const (
	TriggerCompletion = "completion"
	TriggerSignal     = "signal"
	TriggerTime       = "time"
	TriggerChange     = "change"
	TriggerCall       = "call"
)

// SessionAcceptance is what dispatching a signal to an object now would do, read
// from the machines delivery would let take it; where several would, the
// schedule's due order decides which consumes it.
type SessionAcceptance struct {
	// Accepted reports whether a transition of a taking machine is triggered by
	// the signal, whatever its guard; a deferral alone does not set it.
	Accepted bool
	Fires    []SessionTransition
	Deferred bool
	Resumes  []string
}

// Enabled reports whether dispatching the signal would do something with it:
// fire, defer or resume.
func (a SessionAcceptance) Enabled() bool {
	return len(a.Fires) > 0 || a.Deferred || len(a.Resumes) > 0
}

// Taken reports whether a machine of the object would take the signal at all,
// be it to fire, defer, resume, or drop it because every guard is false.
func (a SessionAcceptance) Taken() bool {
	return a.Accepted || a.Enabled()
}

// SessionChoice is one choice a run made, copied from the runtime's note.
type SessionChoice struct {
	Kind         string
	Step         int
	Where        string
	Alternatives []string
	Taken        int
}

// SessionBranch is one way a performance left a decision of the action's own flow.
type SessionBranch struct {
	Decision string
	Target   string
	Else     bool
	Opening  bool
}

// SessionPerformance is what performing an action in a session reported.
type SessionPerformance struct {
	Outputs     map[string]*pb.Value
	Choices     []SessionChoice
	Branches    []SessionBranch
	Diagnostics []*pb.Diagnostic
}

// SessionAdvance is what advancing a session's clock reported.
type SessionAdvance struct {
	From, To    float64
	Events      int64
	Steps       int64
	Choices     []SessionChoice
	Diagnostics []*pb.Diagnostic
}

// SessionMember is one named member a declaration's scope holds.
type SessionMember struct {
	ID   string
	Name string
	Kind string
}

// OpenSession opens a session over a cached model, holding one of the model's
// workers until the session is closed.
func (s *Service) OpenSession(modelHash string) (*Session, error) {
	cached, ok := s.cache.Get(modelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, modelHash)
	}
	w, release := cached.worker()
	rt := s.newRuntimeOver(w)
	rt.SetMaxInstances(s.maxHeldObjects)
	return &Session{svc: s, cached: cached, worker: w, release: release, rt: rt}, nil
}

// Close releases the session's worker; a closed session refuses every call.
func (ss *Session) Close() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.closed {
		return
	}
	ss.closed = true
	ss.rt = nil
	ss.release()
}

// enter takes the session's lock for one call, refusing a closed session.
func (ss *Session) enter() (func(), error) {
	ss.mu.Lock()
	if ss.closed {
		ss.mu.Unlock()
		return nil, statusError(connect.CodeUnavailable, "the session is closed")
	}
	return ss.mu.Unlock, nil
}

// SetSchedule makes the session's later turns follow the scheduling policy: the
// actions it performs from now on, and the machines and clock it already drives
// from their next step on.
func (ss *Session) SetSchedule(spelling string) error {
	done, err := ss.enter()
	if err != nil {
		return err
	}
	defer done()
	policy, err := ss.svc.schedulePolicy(spelling)
	if err != nil {
		return err
	}
	if _, explores := policy.Exploration(); explores {
		return statusErrorf(connect.CodeInvalidArgument,
			"invalid scheduling policy %q: an exploration replays whole runs, which a session does not", spelling)
	}
	if err := ss.rt.Reschedule(policy); err != nil {
		return statusError(connect.CodeInvalidArgument, err.Error())
	}
	return nil
}

// Now is the session's clock.
func (ss *Session) Now() (float64, error) {
	done, err := ss.enter()
	if err != nil {
		return 0, err
	}
	defer done()
	return ss.rt.Clock().Now(), nil
}

// Instantiate instantiates the named part or usage in the session, starting the
// behaviors it exhibits, and answers the object's id.
func (ss *Session) Instantiate(symbolID string) (int64, error) {
	done, err := ss.enter()
	if err != nil {
		return 0, err
	}
	defer done()
	sym, err := ss.declared(symbolID)
	if err != nil {
		return 0, err
	}
	inst, err := ss.rt.Instantiate(sym)
	if err != nil {
		return 0, ss.runFailure(err, "instantiation of %s failed: %v", symbolID)
	}
	return inst.ID, nil
}

// FeatureValue reads one feature of an object the session holds.
func (ss *Session) FeatureValue(object int64, feature string) (*pb.FeatureValue, error) {
	done, err := ss.enter()
	if err != nil {
		return nil, err
	}
	defer done()
	inst, err := ss.object(object)
	if err != nil {
		return nil, err
	}
	fv, err := inst.GetFeatureValue(ss.rt, feature)
	if err != nil {
		return nil, ss.runFailure(err, "feature %s of object %d could not be read: %v", feature, object)
	}
	out := &pb.FeatureValue{FeatureName: feature, Materialized: fv.Materialized}
	if fv.Feature.Scalar() {
		switch {
		case !fv.Materialized:
		case fv.Value.Kind == runtime.ValInvalid:
			out.Value = &pb.Value{Kind: &pb.Value_Unset{Unset: true}}
		default:
			out.Value = ss.svc.valueToProto(ss.rt, fv.Value, ss.cached.Index)
		}
		return out, nil
	}
	for _, elem := range objref.CollectionElements(fv.Values) {
		out.Values = append(out.Values, ss.svc.valueToProto(ss.rt, elem, ss.cached.Index))
	}
	return out, nil
}

// SetFeatureValue writes one feature of an object the session holds.
func (ss *Session) SetFeatureValue(object int64, feature string, value *pb.Value) error {
	done, err := ss.enter()
	if err != nil {
		return err
	}
	defer done()
	inst, err := ss.object(object)
	if err != nil {
		return err
	}
	val, err := ss.value(feature, value)
	if err != nil {
		return err
	}
	if err := inst.SetFeatureValue(ss.rt, feature, val); err != nil {
		return ss.runFailure(err, "feature %s of object %d could not be written: %v", feature, object)
	}
	return nil
}

// Evaluate evaluates one expression in the scope of the named declaration, or
// of the model's primary document when none is named, against the session's state.
func (ss *Session) Evaluate(expression, contextSymbolID string) (*pb.Value, error) {
	done, err := ss.enter()
	if err != nil {
		return nil, err
	}
	defer done()
	exprSource := source.New("<expression>", []byte(expression))
	p := parser.New(exprSource)
	exprNode := p.ParseExpression()
	if len(p.Diagnostics) > 0 {
		var diags []*pb.Diagnostic
		for _, diag := range p.Diagnostics {
			diags = append(diags, ParserDiagnosticToProto(diag, exprSource))
		}
		return nil, &SessionFailure{Message: "expression parse failed", Diagnostics: ss.svc.filterDiagnosticCapabilities(diags)}
	}
	if p.Offset() < len(strings.TrimRight(expression, " \t\r\n")) {
		return nil, sessionFailuref("expression parse failed: unexpected %q after the expression",
			strings.TrimSpace(expression[p.Offset():]))
	}
	scope := ss.cached.PrimaryRoot()
	if contextSymbolID != "" {
		sym, err := ss.declared(contextSymbolID)
		if err != nil {
			return nil, err
		}
		scope = evalScope(sym, ss.cached)
	}
	var result runtime.Value
	var evalErr error
	ss.rt.Resolver().Scratch(astcodec.Reachable(exprNode), func() { result, evalErr = ss.rt.EvalWithScope(exprNode, scope) })
	if evalErr != nil {
		return nil, ss.runFailure(evalErr, "evaluation failed: %v")
	}
	return ss.svc.valueToProto(ss.rt, result, ss.cached.Index), nil
}

// Members lists the named members the named declaration's scope holds, in
// declaration order.
func (ss *Session) Members(symbolID string) ([]SessionMember, error) {
	done, err := ss.enter()
	if err != nil {
		return nil, err
	}
	defer done()
	sym, err := ss.declared(symbolID)
	if err != nil {
		return nil, err
	}
	if sym.Scope == nil {
		return nil, sessionFailuref("%s declares no members", symbolID)
	}
	var members []SessionMember
	for _, member := range sym.Scope.Members() {
		if member.Name == "" {
			continue
		}
		members = append(members, SessionMember{ID: ss.cached.Index.GetFQN(member), Name: member.Name, Kind: member.Kind.String()})
	}
	return members, nil
}

// ActiveStates names the innermost active states of every machine the object
// exhibits, machine by machine in declaration order; the composite states
// enclosing them are active too.
func (ss *Session) ActiveStates(object int64) ([]string, error) {
	done, err := ss.enter()
	if err != nil {
		return nil, err
	}
	defer done()
	machines, err := ss.machines(object)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, machine := range machines {
		for _, state := range machine.ActiveLeaves() {
			names = append(names, state.Name)
		}
	}
	return names, nil
}

// Transitions lists the transitions dispatch could select now in every machine
// the object exhibits: out of each active state and then of each state
// enclosing it, innermost first, in declaration order.
func (ss *Session) Transitions(object int64) ([]SessionTransition, error) {
	done, err := ss.enter()
	if err != nil {
		return nil, err
	}
	defer done()
	machines, err := ss.machines(object)
	if err != nil {
		return nil, err
	}
	var out []SessionTransition
	for _, machine := range machines {
		for _, trans := range machine.OutgoingTransitions() {
			out = append(out, transitionFact(trans))
		}
	}
	return out, nil
}

// transitionFact copies what a transition is, by name, from its lowered form.
func transitionFact(trans *lower.Transition) SessionTransition {
	fact := SessionTransition{
		Name:    trans.Name,
		Source:  runtime.StateVertexName(trans.Source),
		Target:  runtime.StateVertexName(trans.Target),
		Trigger: TriggerCompletion,
		Guarded: trans.Guard != nil,
	}
	switch trigger := trans.Trigger.(type) {
	case *ast.AcceptEvent:
		fact.Trigger = TriggerSignal
		if trigger.SignalType != nil && len(trigger.SignalType.Parts) > 0 {
			fact.Signal = trigger.SignalType.Parts[len(trigger.SignalType.Parts)-1].Text
		} else {
			fact.Event = lower.FeaturePath(trigger.Subsets)
		}
	case *ast.TimeEvent:
		fact.Trigger = TriggerTime
	case *ast.ChangeEvent:
		fact.Trigger = TriggerChange
	case *ast.CallEvent:
		fact.Trigger = TriggerCall
	}
	return fact
}

// Accepts decides what dispatching the signal to the object now would do,
// without dispatching it.
func (ss *Session) Accepts(object int64, signalID string, args map[string]*pb.Value) (*SessionAcceptance, error) {
	done, err := ss.enter()
	if err != nil {
		return nil, err
	}
	defer done()
	machines, msg, err := ss.signal(object, signalID, args)
	if err != nil {
		return nil, err
	}
	return ss.decide(machines, msg)
}

// decide previews the message on each machine delivery would let take it — one
// that reacts to it and does not yield it to a sibling — in exhibit order.
func (ss *Session) decide(machines []*runtime.StateExecutor, msg runtime.Message) (*SessionAcceptance, error) {
	out := &SessionAcceptance{}
	for _, machine := range machines {
		takes, err := machine.TakesMessage(msg)
		if err != nil {
			return nil, ss.runFailure(err, "deciding the signal failed: %v")
		}
		if !takes {
			continue
		}
		triggered, err := machine.TriggeredBy(msg)
		if err != nil {
			return nil, ss.runFailure(err, "deciding the signal failed: %v")
		}
		out.Accepted = out.Accepted || triggered
		decision, transitions, err := machine.DecideTransitions(msg)
		if err != nil {
			return nil, ss.runFailure(err, "deciding the signal failed: %v")
		}
		for _, trans := range transitions {
			out.Fires = append(out.Fires, transitionFact(trans))
		}
		out.Deferred = out.Deferred || decision.Deferred
		out.Resumes = append(out.Resumes, decision.Resumes...)
	}
	return out, nil
}

// Send posts the signal to the object, refusing one no machine would take or
// one every taking machine would drop; Advance then dispatches it.
func (ss *Session) Send(object int64, signalID string, args map[string]*pb.Value) (*SessionAcceptance, error) {
	done, err := ss.enter()
	if err != nil {
		return nil, err
	}
	defer done()
	machines, msg, err := ss.signal(object, signalID, args)
	if err != nil {
		return nil, err
	}
	acceptance, err := ss.decide(machines, msg)
	if err != nil {
		return nil, err
	}
	if !acceptance.Taken() {
		return nil, statusErrorf(connect.CodeFailedPrecondition, "no active state accepts or defers %s", signalID)
	}
	if !acceptance.Enabled() {
		return nil, statusErrorf(connect.CodeFailedPrecondition, "the active states accept %s but no guard on it holds", signalID)
	}
	ss.rt.PostMessage(msg)
	return acceptance, nil
}

// signal builds the message the signal definition sends to the object.
func (ss *Session) signal(object int64, signalID string, args map[string]*pb.Value) ([]*runtime.StateExecutor, runtime.Message, error) {
	inst, err := ss.object(object)
	if err != nil {
		return nil, runtime.Message{}, err
	}
	machines, err := ss.machinesOf(inst, object)
	if err != nil {
		return nil, runtime.Message{}, err
	}
	sym, err := ss.declared(signalID)
	if err != nil {
		return nil, runtime.Message{}, err
	}
	if !runtime.IsSignalDefinition(sym) {
		return nil, runtime.Message{}, sessionFailuref("%s is not a signal definition", signalID)
	}
	values, err := ss.values(args)
	if err != nil {
		return nil, runtime.Message{}, err
	}
	msg, err := ss.rt.SignalMessage(sym, values, inst)
	if err != nil {
		return nil, runtime.Message{}, ss.runFailure(err, "signal %s could not be built: %v", signalID)
	}
	return machines, msg, nil
}

// Advance moves the session's clock by seconds, dispatching what is due.
func (ss *Session) Advance(seconds float64) (*SessionAdvance, error) {
	done, err := ss.enter()
	if err != nil {
		return nil, err
	}
	defer done()
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return nil, statusErrorf(connect.CodeInvalidArgument, "the clock advances by a finite non-negative duration, not %v", seconds)
	}
	report, err := ss.rt.Advance(seconds)
	if err != nil {
		if errors.Is(err, runtime.ErrInstanceLimitExceeded) {
			return nil, ss.runFailure(err, "advance failed: %v")
		}
		return nil, &SessionFailure{Message: fmt.Sprintf("advance failed: %v", err), Diagnostics: ss.diagnostics(report.Notes)}
	}
	return &SessionAdvance{
		From:        report.From,
		To:          report.To,
		Events:      report.Events,
		Steps:       report.Steps,
		Choices:     choicesOf(report.Notes),
		Diagnostics: ss.diagnostics(report.Notes),
	}, nil
}

// Perform performs the named action on the object with the inputs, running it
// to completion, and reports what the run made and decided.
func (ss *Session) Perform(object int64, actionID string, inputs map[string]*pb.Value) (*SessionPerformance, error) {
	done, err := ss.enter()
	if err != nil {
		return nil, err
	}
	defer done()
	inst, err := ss.object(object)
	if err != nil {
		return nil, err
	}
	sym, err := ss.declared(actionID)
	if err != nil {
		return nil, err
	}
	values, err := ss.values(inputs)
	if err != nil {
		return nil, err
	}
	exec, err := ss.rt.CreateActionExecutorWithInputs(sym, inst, values)
	if err != nil {
		return nil, ss.runFailure(err, "action %s could not be performed: %v", actionID)
	}
	defer exec.Release()
	exec.KeepTraversals(true)
	if err := exec.RunToCompletion(); err != nil {
		if errors.Is(err, runtime.ErrInstanceLimitExceeded) {
			return nil, ss.runFailure(err, "action %s failed: %v", actionID)
		}
		return nil, &SessionFailure{Message: fmt.Sprintf("action %s failed: %v", actionID, err), Diagnostics: ss.diagnostics(exec.Notes())}
	}
	notes := exec.Notes()
	out := &SessionPerformance{
		Outputs:     make(map[string]*pb.Value),
		Choices:     choicesOf(notes),
		Branches:    branchesOf(exec),
		Diagnostics: ss.diagnostics(notes),
	}
	for name, val := range exec.Results() {
		out.Outputs[name] = ss.svc.valueToProto(ss.rt, val, ss.cached.Index)
	}
	return out, nil
}

// branchesOf lists the decisions the action's own flow left, in traversal order.
func branchesOf(exec *runtime.ActionExecutor) []SessionBranch {
	graph := exec.Graph()
	gate := gateOf(graph)
	var out []SessionBranch
	for _, t := range exec.Traversals() {
		if len(t.Within) > 0 {
			continue
		}
		if _, ok := t.Edge.Source.(*ast.DecisionNode); !ok {
			continue
		}
		branch := SessionBranch{
			Decision: runtime.ActionNodeName(t.Edge.Source),
			Target:   runtime.ActionNodeName(t.Edge.Target),
			Opening:  t.Edge.Source == gate,
		}
		if edge, ok := t.Edge.Decl.(*ast.ControlFlowEdge); ok {
			branch.Else = edge.IsElse
		}
		out = append(out, branch)
	}
	return out
}

// gateOf is the first decision the action's start leads to without a choice on
// the way, following each node's single succession; nil when there is none.
func gateOf(graph *lower.ActionGraph) ast.Node {
	var at ast.Node
	for _, node := range graph.Nodes {
		if _, ok := node.(*ast.InitialNode); ok {
			at = node
			break
		}
	}
	seen := map[ast.Node]bool{}
	for at != nil && !seen[at] {
		seen[at] = true
		if _, ok := at.(*ast.DecisionNode); ok {
			return at
		}
		edges := graph.Edges[at]
		if len(edges) != 1 {
			return nil
		}
		at = edges[0].Target
	}
	return nil
}

// choicesOf copies the choice points among a run's notes.
func choicesOf(notes []runtime.RunNote) []SessionChoice {
	var out []SessionChoice
	for _, note := range notes {
		c, ok := note.(runtime.ChoicePoint)
		if !ok {
			continue
		}
		out = append(out, SessionChoice{
			Kind:         c.Kind.String(),
			Step:         c.Step,
			Where:        c.Where,
			Alternatives: append([]string(nil), c.Alternatives...),
			Taken:        c.Taken,
		})
	}
	return out
}

// diagnostics reports a run's notes as diagnostics, under the service's capabilities.
func (ss *Session) diagnostics(notes []runtime.RunNote) []*pb.Diagnostic {
	return ss.svc.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(notes, ss.cached))
}

// declared resolves a symbol id in the session's model.
func (ss *Session) declared(symbolID string) (*symbols.Symbol, error) {
	syms := lookupNamed(ss.cached.Index, symbolID)
	if len(syms) == 0 {
		return nil, sessionFailuref("symbol not found: %s", symbolID)
	}
	return syms[0], nil
}

// object dereferences an object id the session handed out.
func (ss *Session) object(id int64) (*runtime.Instance, error) {
	inst, ok := ss.rt.Instance(id)
	if !ok {
		return nil, sessionFailuref("object not found: %d", id)
	}
	return inst, nil
}

// machines are the state machines the object exhibits, in declaration order.
func (ss *Session) machines(object int64) ([]*runtime.StateExecutor, error) {
	inst, err := ss.object(object)
	if err != nil {
		return nil, err
	}
	return ss.machinesOf(inst, object)
}

func (ss *Session) machinesOf(inst *runtime.Instance, object int64) ([]*runtime.StateExecutor, error) {
	var machines []*runtime.StateExecutor
	for _, exhibited := range inst.ExhibitedStates() {
		if exhibited.State != nil {
			machines = append(machines, exhibited.State)
		}
	}
	if len(machines) == 0 {
		return nil, sessionFailuref("object %d exhibits no state machine", object)
	}
	return machines, nil
}

// value reads one named wire value as the session's runtime holds it; a value of
// a kind the service lacks the capability for is a status, an unreadable one a failure.
func (ss *Session) value(name string, pv *pb.Value) (runtime.Value, error) {
	if err := ss.svc.requireValueCapabilities(pv); err != nil {
		return runtime.Value{}, err
	}
	val, err := protoconv.ProtoToRuntimeValue(ss.rt, pv, ss.cached.Index, ss.rt.Semantics())
	if err != nil {
		return runtime.Value{}, sessionFailuref("value %q could not be read: %v", name, err)
	}
	return val, nil
}

// values reads named wire values as the session's runtime holds them.
func (ss *Session) values(pvs map[string]*pb.Value) (map[string]runtime.Value, error) {
	if len(pvs) == 0 {
		return nil, nil
	}
	out := make(map[string]runtime.Value, len(pvs))
	for name, pv := range pvs {
		val, err := ss.value(name, pv)
		if err != nil {
			return nil, err
		}
		out[name] = val
	}
	return out, nil
}
