package opensysml

import (
	"sync"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
)

// Session is a persistent, interactive run of one model: it keeps a clock, a
// scheduling policy and the objects it instantiated across calls, so a caller
// can instantiate a part, send its state machine a signal, perform an action
// on it and read what changed, one step at a time. Sessions are answered in
// process only — see OpenSession — and are not part of the Client interface.
// A Session is safe for concurrent use; every call is answered before it
// returns, bounded by the same step budget as ExecuteAction.
type Session struct {
	mu     sync.Mutex
	engine *sysmlgrpc.Session
	closed bool
}

// OpenSession opens a Session over a model the client parsed. Only a client
// New returned can answer: a session is state the engine holds between calls,
// which the service exposes no RPC for, so a Dial client answers
// CodeUnimplemented. The model must still be in the client's cache
// (CodeNotFound otherwise); the session then holds it for as long as it lives.
func OpenSession(c Client, model *Model) (*Session, error) {
	cl, ok := c.(*client)
	if !ok {
		return nil, &StatusError{Code: CodeInvalidArgument, Message: "OpenSession takes a client New or Dial returned"}
	}
	hash, err := cl.call(model)
	if err != nil {
		return nil, err
	}
	local, ok := cl.caller.(*inprocess)
	if !ok {
		return nil, &StatusError{
			Code:    CodeUnimplemented,
			Message: "persistent sessions are answered in process only: open one from a client New returned",
		}
	}
	engine, err := local.svc.OpenSession(hash)
	if err != nil {
		return nil, statusToError(err)
	}
	return &Session{engine: engine}, nil
}

// Close ends the session and releases what it holds. Every later call answers
// CodeUnavailable; closing twice is harmless.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.engine.Close()
	return nil
}

// live refuses a closed session.
func (s *Session) live() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return &StatusError{Code: CodeUnavailable, Message: "the session is closed"}
	}
	return nil
}

// answer runs one engine call behind the public boundary: a closed session, a
// status or a panic becomes a StatusError, the model's own failure a FailureError.
func (s *Session) answer(op string, call func() error) (err error) {
	if err := s.live(); err != nil {
		return err
	}
	defer recoverToError(&err)
	return sessionError(op, call())
}

// sessionError maps what the engine answered to the documented error types.
func sessionError(op string, err error) error {
	if err == nil {
		return nil
	}
	if failure, ok := err.(*sysmlgrpc.SessionFailure); ok {
		return &FailureError{Op: op, Message: failure.Message, Diagnostics: diagnosticsFromProto(failure.Diagnostics)}
	}
	return statusToError(err)
}

// SetSchedule makes the session's later runs resolve their choice points under
// the policy, as sysml -schedule spells it ("seed:42"); "explore[...]" is
// refused, an exploration replays whole runs. Setting the same seed again
// restarts its stream, so two runs from the same seed choose alike.
func (s *Session) SetSchedule(policy string) error {
	return s.answer("SetSchedule", func() error { return s.engine.SetSchedule(policy) })
}

// Now is the session's clock, in seconds since it opened.
func (s *Session) Now() (now float64, err error) {
	err = s.answer("Now", func() error {
		now, err = s.engine.Now()
		return err
	})
	return now, err
}

// Instantiate creates an object of the named part or usage, starting the state
// machines it exhibits, and answers the handle later calls take. The object
// lives until the session closes.
func (s *Session) Instantiate(symbolID string) (id InstanceID, err error) {
	err = s.answer("Instantiate", func() error {
		raw, err := s.engine.Instantiate(symbolID)
		id = InstanceID(raw)
		return err
	})
	return id, err
}

// Feature reads what an object holds for one feature of its type, by name, as
// the session's runs have left it.
func (s *Session) Feature(object InstanceID, name string) (value *FeatureValue, err error) {
	err = s.answer("Feature", func() error {
		fv, err := s.engine.FeatureValue(int64(object), name)
		if err != nil {
			return err
		}
		value = &FeatureValue{FeatureName: fv.FeatureName, Value: valueFromProto(fv.Value), Materialized: fv.Materialized}
		for _, elem := range fv.Values {
			value.Values = append(value.Values, valueFromProto(elem))
		}
		return nil
	})
	return value, err
}

// SetFeature writes one feature of an object, as an assignment in the model would.
func (s *Session) SetFeature(object InstanceID, name string, value Value) error {
	return s.answer("SetFeature", func() error {
		sent, err := valueToProto(value)
		if err != nil {
			return err
		}
		return s.engine.SetFeatureValue(int64(object), name, sent)
	})
}

// Evaluate evaluates one expression against the session's state, resolving its
// names in the scope WithContextSymbol names, or the model's primary document
// when none is given. A feature of an object the session holds reads that
// object's current value. WithSubject is not taken: the session's objects are
// the subjects.
func (s *Session) Evaluate(expression string, opts ...EvaluateOption) (value Value, err error) {
	var options evaluateOptions
	for _, opt := range opts {
		opt(&options)
	}
	if options.subjectSymbolID != "" {
		return nil, &StatusError{Code: CodeInvalidArgument, Message: "Session.Evaluate takes no subject: instantiate one and read its features instead"}
	}
	err = s.answer("Evaluate", func() error {
		raw, err := s.engine.Evaluate(expression, options.contextSymbolID)
		if err != nil {
			return err
		}
		value = valueFromProto(raw)
		return nil
	})
	return value, err
}

// Member is one named member a declaration's scope holds.
type Member struct {
	// ID is the member's fully qualified name.
	ID   string
	Name string
	// Kind of the member ("PartUsage", "EnumerationUsage", …).
	Kind string
}

// Members lists the named members of the named package, definition or usage,
// in declaration order: the literals of an enumeration, the parts of a package.
func (s *Session) Members(symbolID string) (members []Member, err error) {
	err = s.answer("Members", func() error {
		facts, err := s.engine.Members(symbolID)
		if err != nil {
			return err
		}
		for _, m := range facts {
			members = append(members, Member{ID: m.ID, Name: m.Name, Kind: m.Kind})
		}
		return nil
	})
	return members, err
}

// TriggerKind says what fires a Transition.
type TriggerKind string

// The kinds of trigger a Transition reports.
const (
	// TriggerCompletion fires when the source state's entry behavior completes.
	TriggerCompletion TriggerKind = sysmlgrpc.TriggerCompletion
	// TriggerSignal fires on accepting a signal: one of the type
	// Transition.Signal names, or an occurrence of the event feature
	// Transition.Event names.
	TriggerSignal TriggerKind = sysmlgrpc.TriggerSignal
	// TriggerTime fires when a time event is due.
	TriggerTime TriggerKind = sysmlgrpc.TriggerTime
	// TriggerChange fires when a change event's condition comes true.
	TriggerChange TriggerKind = sysmlgrpc.TriggerChange
	// TriggerCall fires on a call event.
	TriggerCall TriggerKind = sysmlgrpc.TriggerCall
)

// Transition is one transition of a state machine, as declared: which states it
// joins, what fires it and whether a guard stands on it.
type Transition struct {
	// Name of the transition, "" when it was declared anonymously.
	Name string
	// Source and Target name the states the transition joins.
	Source string
	Target string
	// Trigger says what fires it.
	Trigger TriggerKind
	// Signal is the simple name of the signal type a TriggerSignal transition
	// accepts, "" when it accepts by event feature instead or for the other kinds.
	Signal string
	// Event is the feature path an accept trigger subsets (`alert`,
	// `left.alert`) when it accepts an occurrence of that event feature rather
	// than a signal type, "" otherwise.
	Event string
	// Guarded reports whether a guard stands on it; Accepts says whether the
	// guard holds now.
	Guarded bool
}

func transitionFromFact(t sysmlgrpc.SessionTransition) Transition {
	return Transition{Name: t.Name, Source: t.Source, Target: t.Target, Trigger: TriggerKind(t.Trigger), Signal: t.Signal, Event: t.Event, Guarded: t.Guarded}
}

// ActiveStates names the innermost active states of every state machine the
// object exhibits, machine by machine in declaration order, one per active
// region; the composite states enclosing them are active too. A FailureError
// when the object exhibits none.
func (s *Session) ActiveStates(object InstanceID) (states []string, err error) {
	err = s.answer("ActiveStates", func() error {
		states, err = s.engine.ActiveStates(int64(object))
		return err
	})
	return states, err
}

// Transitions lists the transitions dispatch could select now, machine by
// machine: those out of each active state and then of each state enclosing
// it, innermost first as dispatch tries them, each in declaration order. A
// caller offers them as what the object could do next; Source tells which
// state declares each.
func (s *Session) Transitions(object InstanceID) (transitions []Transition, err error) {
	err = s.answer("Transitions", func() error {
		facts, err := s.engine.Transitions(int64(object))
		if err != nil {
			return err
		}
		for _, t := range facts {
			transitions = append(transitions, transitionFromFact(t))
		}
		return nil
	})
	return transitions, err
}

// Acceptance is what dispatching a signal to an object would do now.
type Acceptance struct {
	// Accepted reports whether a transition out of an active state is
	// triggered by the signal at all, whatever its guard.
	Accepted bool
	// Fires lists the transitions that would fire, in the order they would.
	Fires []Transition
	// Deferred reports whether the active state defers the signal.
	Deferred bool
	// Resumes names the states a deferred signal would resume.
	Resumes []string
}

// Enabled reports whether the machine would do something with the signal: a
// transition fires, or the signal is deferred or resumes a state. An accepted
// signal that is not enabled is one whose every transition's guard is false.
func (a *Acceptance) Enabled() bool {
	return a != nil && (len(a.Fires) > 0 || a.Deferred || len(a.Resumes) > 0)
}

func acceptanceFromFact(a *sysmlgrpc.SessionAcceptance) *Acceptance {
	out := &Acceptance{Accepted: a.Accepted, Deferred: a.Deferred, Resumes: append([]string(nil), a.Resumes...)}
	for _, t := range a.Fires {
		out.Fires = append(out.Fires, transitionFromFact(t))
	}
	return out
}

// Accepts says what sending the signal to the object would do now, without
// sending it: whether a transition of any machine it exhibits, an enclosing
// state's included, accepts it and whether its guard holds. The signal
// definition is named by ID; args bind its attributes.
func (s *Session) Accepts(object InstanceID, signalID string, args map[string]Value) (acceptance *Acceptance, err error) {
	err = s.answer("Accepts", func() error {
		sent, err := valuesToProto(args)
		if err != nil {
			return err
		}
		fact, err := s.engine.Accepts(int64(object), signalID, sent)
		if err != nil {
			return err
		}
		acceptance = acceptanceFromFact(fact)
		return nil
	})
	return acceptance, err
}

// Send posts the signal to the object; Advance then dispatches it and runs
// what follows, completion transitions included. A signal no transition out of
// an active state or a state enclosing one accepts, in any machine the object
// exhibits, or one whose every guard is false, is refused with
// CodeFailedPrecondition and nothing is posted.
func (s *Session) Send(object InstanceID, signalID string, args map[string]Value) (acceptance *Acceptance, err error) {
	err = s.answer("Send", func() error {
		sent, err := valuesToProto(args)
		if err != nil {
			return err
		}
		fact, err := s.engine.Send(int64(object), signalID, sent)
		if err != nil {
			return err
		}
		acceptance = acceptanceFromFact(fact)
		return nil
	})
	return acceptance, err
}

// ChoicePoint is one choice a run made: where it stood, what it could have
// chosen and what the scheduling policy chose.
type ChoicePoint struct {
	// Kind of choice ("decision branch", "fork order", …).
	Kind string
	// Step of the run the choice was made at.
	Step int
	// Where names the node the choice was made at, as the run traces it
	// ("decision roll").
	Where string
	// Alternatives are what could have been chosen, in declaration order.
	Alternatives []string
	// Taken indexes Alternatives with what was chosen.
	Taken int
}

// ChoiceDecisionBranch is the Kind of a ChoicePoint made at a decision node.
const ChoiceDecisionBranch = "decision branch"

func choicesFromFacts(facts []sysmlgrpc.SessionChoice) []ChoicePoint {
	var out []ChoicePoint
	for _, c := range facts {
		out = append(out, ChoicePoint{Kind: c.Kind, Step: c.Step, Where: c.Where, Alternatives: append([]string(nil), c.Alternatives...), Taken: c.Taken})
	}
	return out
}

// Advancement is what advancing the clock did.
type Advancement struct {
	// From and To are the clock before and after.
	From, To float64
	// Events dispatched and Steps run while advancing.
	Events, Steps int64
	// Choices the runs made while advancing.
	Choices []ChoicePoint
	// Diagnostics the runs raised.
	Diagnostics []Diagnostic
}

// Advance moves the clock forward by seconds — zero to dispatch what is posted
// now — running the transitions, effects and completion transitions that
// follow until the machines settle.
func (s *Session) Advance(seconds float64) (advanced *Advancement, err error) {
	err = s.answer("Advance", func() error {
		fact, err := s.engine.Advance(seconds)
		if err != nil {
			return err
		}
		advanced = &Advancement{
			From:        fact.From,
			To:          fact.To,
			Events:      fact.Events,
			Steps:       fact.Steps,
			Choices:     choicesFromFacts(fact.Choices),
			Diagnostics: diagnosticsFromProto(fact.Diagnostics),
		}
		return nil
	})
	return advanced, err
}

// Branch is one way a performance left a decision node of the action's own
// flow — not of the actions it called.
type Branch struct {
	// Decision names the decision node, "" when it is anonymous.
	Decision string
	// Target names the node the branch led to.
	Target string
	// Else reports the branch was the decision's `else`, taken when no guarded
	// branch was.
	Else bool
	// Opening reports the decision is the action's gate: the first decision
	// its start leads to with no other choice on the way.
	Opening bool
}

// Performance is what performing an action did.
type Performance struct {
	// Outputs are the action's out parameters, by name.
	Outputs map[string]Value
	// Choices the run made, in order.
	Choices []ChoicePoint
	// Branches are the decisions of the action's own flow the run left, in order.
	Branches []Branch
	// Diagnostics the run raised.
	Diagnostics []Diagnostic
}

// TurnedAway reports whether the run left the action's opening decision by its
// else branch: the action looked at its inputs or its performer and declined.
func (p *Performance) TurnedAway() bool {
	if p == nil {
		return false
	}
	for _, b := range p.Branches {
		if b.Opening && b.Else {
			return true
		}
	}
	return false
}

// Perform performs the named action on the object, binding inputs to its in
// parameters, and runs it to completion in the session's state; the object's
// features read and written by the action are those the session holds. A run
// the model fails is a FailureError carrying the run's diagnostics.
func (s *Session) Perform(object InstanceID, actionID string, inputs map[string]Value) (performed *Performance, err error) {
	err = s.answer("Perform", func() error {
		sent, err := valuesToProto(inputs)
		if err != nil {
			return err
		}
		fact, err := s.engine.Perform(int64(object), actionID, sent)
		if err != nil {
			return err
		}
		performed = &Performance{
			Outputs:     valuesFromProto(fact.Outputs),
			Choices:     choicesFromFacts(fact.Choices),
			Diagnostics: diagnosticsFromProto(fact.Diagnostics),
		}
		for _, b := range fact.Branches {
			performed.Branches = append(performed.Branches, Branch{Decision: b.Decision, Target: b.Target, Else: b.Else, Opening: b.Opening})
		}
		return nil
	})
	return performed, err
}

// valuesToProto marshals named values for the engine; a value that cannot be
// sent is the StatusError valueToProto refuses it with.
func valuesToProto(values map[string]Value) (map[string]*pb.Value, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make(map[string]*pb.Value, len(values))
	for name, value := range values {
		sent, err := valueToProto(value)
		if err != nil {
			return nil, err
		}
		out[name] = sent
	}
	return out, nil
}
