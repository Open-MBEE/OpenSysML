package runtime

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// StateConfiguration represents the active state configuration (simple or multi-region).
type StateConfiguration struct {
	// For simple states (no regions): single active state
	simpleState *ast.StateNode

	// For composite states with regions: map of region → active state in that region
	regionStates map[*ast.StateRegion]*ast.StateNode
}

// StateExecutor executes state machines using event-driven semantics.
type StateExecutor struct {
	ctx          *Context
	stateMachine *symbols.Symbol
	// self is the object performing the machine: its connections route what the
	// machine sends, and its selections decide which variant's connection does.
	self *Instance
	// occurrence is the state performance materialized for an exhibited usage.
	occurrence *Instance
	state      ExecutionState

	// Lowered graph (source of truth)
	graph *lower.StateGraph

	// State machine execution state
	activeConfig *StateConfiguration // Active state configuration (simple or multi-region)
	nextEventID  int64               // Monotonic counter for unique event IDs
	// eventQueue holds this machine's events by instant of the context's clock; a
	// timer set for later is this machine's wait on the clock.
	eventQueue *EventQueue
	stateData  map[string]Value // State machine local variables
	// stateAttrs holds the attributes each state owns, one map per state node, so
	// two usages of one state definition keep separate values.
	stateAttrs  map[*ast.StateNode]map[string]Value
	stateVisits []string         // Ordered list of visited state names
	stateStack  []*ast.StateNode // Active state configuration (for nested states)

	// history records, per composite state, the configuration that state had when
	// it was last exited. A history pseudostate re-enters that configuration
	// instead of the composite state's initial one.
	history map[*ast.StateNode]*historyRecord

	// deferred holds, in arrival order, the events an active state defers and no
	// transition of the active configuration handled.
	deferred []Event
	// lastDispatch is what became of the event the last step took off the queue,
	// lastEventAt the instant it was dispatched at.
	lastDispatch *Dispatch
	lastEventAt  float64

	// fired are the transitions taken and still kept, in firing order, firedBase
	// those released before them; a dispatch keeps its own unless keepFired is set.
	fired     []FiredTransition
	firedBase int
	keepFired bool

	// breakpointNodes are the vertices a run pauses on entering or passing through;
	// breakpointHit the first state the dispatch under way entered, pausedAt the
	// vertex the run paused at.
	breakpointNodes map[ast.Node]bool
	breakpointHit   *ast.StateNode
	pausedAt        ast.Node
	// dispatchMark is where the dispatch under way began in fired, -1 between
	// dispatches; completionDue holds a completion reached under a breakpoint.
	dispatchMark  int
	completionDue bool

	// doActions are the running do behaviors, in the order their states were
	// entered. Concurrently active states interleave one action per round, so this
	// order — not map iteration order — decides the interleaving.
	doActions []*doAction
	// round is the do round under way when the machine runs one unit at a time: the
	// do actions still to act in it; roundDone marks a round closed, its dispatch owed.
	round     []*doAction
	roundDone bool
	// machineExited prevents a parallel machine's root exit behavior from
	// running more than once if completion is reported by multiple regions.
	machineExited bool

	// driven is this executor's run over however many calls drive it: begun once,
	// so the step budget is reset once, and keeping its scheduler throughout.
	driven executorRun
	// inRun is set while a run loop of this machine is on the stack.
	inRun bool
	// moved is set once an event was queued or dispatched, a transition fired, a do
	// step ran or a value written, and cleared when the start attaching it settles.
	moved bool

	// timerScheduled holds the time-triggered transitions whose timer is already
	// running, so a state's timer is not restarted while it stays active.
	timerScheduled map[*lower.Transition]bool
	// timeTriggerVerdict caches, per time-triggered transition, whether its
	// argument's static type was accepted (nil) or the error refusing it.
	timeTriggerVerdict map[*lower.Transition]error

	// changeFired holds the change-triggered transitions already taken on a
	// condition that has stayed true, so an unchanged one does not re-fire.
	changeFired map[*lower.Transition]bool

	// firingChange is the change-triggered transition being taken, whose latch the
	// state entries it causes must leave alone.
	firingChange *lower.Transition

	// firingNotes is what selecting the transition being taken noted, recorded
	// once its guard's final reading lets it fire (see transitionDecided).
	firingNotes []RunNote
	// firingEvent is the occurrence the transition being taken reacts to, for the
	// other segments into a join it fires to bind their own trigger's arguments.
	firingEvent *Event

	// leftAhead are the states a compound transition under way left before its
	// choice was resolved; exitingAhead is set while it leaves them. Both live
	// within one firing, inside a step, so no snapshot sees them.
	leftAhead    map[*ast.StateNode]bool
	exitingAhead bool
	// enteredAhead are the states a move activated ahead of entering them, so a
	// segment's effect could follow the entry of the state enclosing it.
	enteredAhead map[*ast.StateNode]bool
	// moving marks a compound transition under way that a refused witness may undo.
	moving *moveMark

	// changeRearmed collects, while a poll runs, the watches a state entry armed
	// for a new activation, so the poll's earlier observation does not latch them.
	changeRearmed map[*lower.Transition]bool

	// changeWaits are the change conditions the last poll found could not fire,
	// telling a machine waiting on one from a quiesced machine.
	changeWaits []changeWait
}

// doAction is the part of a state's do behavior that has still to run. The
// behavior runs while its state is active rather than at entry, and is abandoned
// when the state is exited.
type doAction struct {
	state   *ast.StateNode
	pending []lower.StateBehavior
	// run is the behavior under way, paused where its flow waits on the clock or
	// for a message; nil between behaviors.
	run *doRun
}

// due reports work of the do behavior runnable now: a paused behavior whose wait
// has ended, or the next behavior where none is under way.
func (act *doAction) due(ctx *Context) bool {
	if act.run != nil {
		return act.run.resumable(ctx)
	}
	return len(act.pending) > 0
}

// finished reports a do behavior with nothing left to run.
func (act *doAction) finished() bool {
	return act.run == nil && len(act.pending) == 0
}

// historyRecord is the configuration one composite state was last left in.
type historyRecord struct {
	// child is the substate that was active, whether it was declared directly or
	// in one of the state's orthogonal regions.
	child *ast.StateNode
	// regions is the active state of each orthogonal region, empty for a state
	// that has none.
	regions map[*ast.StateRegion]*ast.StateNode
}

// newStateExecutor creates a state executor. self is the object performing the
// machine, nil for a machine no object performs.
func newStateExecutor(ctx *Context, stateMachine *symbols.Symbol, self *Instance) (*StateExecutor, error) {
	return newStateExecutorForOccurrence(ctx, stateMachine, self, nil)
}

func newStateExecutorForOccurrence(
	ctx *Context,
	stateMachine *symbols.Symbol,
	self *Instance,
	occurrence *Instance,
) (*StateExecutor, error) {
	if stateMachine.Kind != symbols.SymbolStateUsage && stateMachine.Kind != symbols.SymbolStateDef {
		return nil, fmt.Errorf("symbol %s is not a state machine", stateMachine.Name)
	}
	if err := ctx.checkPerformer(self); err != nil {
		return nil, err
	}

	// Lower to StateGraph, in the scope the machine's body was written in, so
	// that everything the graph carries is evaluated where it was declared.
	// Endpoints come from the name-resolution tier, which reported on them already.
	graph, err := lower.ToStateGraphWithEndpoints(stateMachine.Decl, DeclScope(stateMachine), lower.NewLibraryStateTypes(ctx.model.resolver))
	if err != nil {
		return nil, fmt.Errorf("lower state machine: %w", err)
	}
	exec := newStateExecutorOn(ctx, stateMachine, self, occurrence, graph)

	// Initialize state machine attributes
	if err := exec.initializeAttributes(); err != nil {
		return nil, err
	}
	if err := exec.initializeStateAttributes(); err != nil {
		return nil, err
	}
	ctx.clock.attach(exec)

	return exec, nil
}

// newStateExecutorOn is an execution of graph, the lowering of stateMachine, holding
// no attribute values yet and not on ctx's clock.
func newStateExecutorOn(
	ctx *Context,
	stateMachine *symbols.Symbol,
	self, occurrence *Instance,
	graph *lower.StateGraph,
) *StateExecutor {
	exec := &StateExecutor{
		ctx:                ctx,
		stateMachine:       stateMachine,
		self:               self,
		occurrence:         occurrence,
		state:              StateReady,
		graph:              graph,
		nextEventID:        1,
		eventQueue:         NewEventQueue(),
		stateData:          make(map[string]Value),
		stateAttrs:         make(map[*ast.StateNode]map[string]Value),
		stateVisits:        make([]string, 0),
		stateStack:         make([]*ast.StateNode, 0),
		history:            make(map[*ast.StateNode]*historyRecord),
		deferred:           make([]Event, 0),
		timerScheduled:     make(map[*lower.Transition]bool),
		timeTriggerVerdict: make(map[*lower.Transition]error),
		changeFired:        make(map[*lower.Transition]bool),
		breakpointNodes:    make(map[ast.Node]bool),
		dispatchMark:       -1,
		activeConfig: &StateConfiguration{
			regionStates: make(map[*ast.StateRegion]*ast.StateNode),
		},
	}
	exec.driven.exec = exec
	return exec
}

// initializeAttributes populates stateData from the exhibited occurrence, or
// from declared defaults when the machine has no occurrence.
func (e *StateExecutor) initializeAttributes() error {
	if e.occurrence != nil {
		for _, attr := range e.graph.Attributes {
			fv, err := e.occurrence.GetFeatureValue(e.ctx, attr.Name)
			if err != nil {
				return fmt.Errorf("%w: read %s of object #%d: %w",
					ErrStatePerformanceOccurrence, attr.Name, e.occurrence.ID, err)
			}
			if value := fv.HeldValue(); value.Kind != ValInvalid {
				e.stateData[attr.Name] = value
			}
		}
		return nil
	}

	ec := NewEvalContextIn(e.ctx, e.graph.Scope, e.self)
	defer ec.beginStep()()
	for _, attr := range e.graph.Attributes {
		if attr.Value == nil {
			continue
		}
		value, err := ec.Eval(attr.Value)
		if err != nil {
			return fmt.Errorf("eval attribute default %s: %w", attr.Name, err)
		}
		e.stateData[attr.Name] = value
	}

	return nil
}

// initializeStateAttributes gives every state that owns attributes its own
// values, so two usages of one state definition never share them.
func (e *StateExecutor) initializeStateAttributes() error {
	for state, attrs := range e.graph.StateAttributes {
		if len(attrs) == 0 {
			continue
		}
		data := make(map[string]Value, len(attrs))
		e.stateAttrs[state] = data
		for _, attr := range attrs {
			if attr.Value == nil {
				continue
			}
			scope := attr.Scope
			if scope == nil {
				scope = e.graph.Scope
			}
			ec := NewEvalContextIn(e.ctx, scope, e.self)
			end := ec.beginStep()
			value, err := ec.Eval(attr.Value)
			end()
			if err != nil {
				return fmt.Errorf("eval attribute default %s of state %s: %w", attr.Name, state.Name, err)
			}
			data[attr.Name] = value
		}
	}
	return nil
}

// attrFramesFor are the attribute values a behavior of state reads, outermost
// state first so an inner state's attribute shadows an enclosing one's.
func (e *StateExecutor) attrFramesFor(state *ast.StateNode) []map[string]Value {
	if state == nil || len(e.stateAttrs) == 0 {
		return nil
	}
	var frames []map[string]Value
	chain := e.getParentChain(state)
	for i := len(chain) - 1; i >= 0; i-- {
		if data := e.stateAttrs[chain[i]]; data != nil {
			frames = append(frames, data)
		}
	}
	return frames
}

// stateAttributeValues is the value map of the innermost state at or enclosing
// state that owns an attribute of this name, with the scope that attribute is
// declared in, so a write to it answers to its declaration.
func (e *StateExecutor) stateAttributeValues(state *ast.StateNode, name string) (map[string]Value, *symbols.Scope, bool) {
	if state == nil {
		return nil, nil, false
	}
	for _, ancestor := range e.getParentChain(state) {
		data, ok := e.stateAttrs[ancestor]
		if !ok {
			continue
		}
		for _, attr := range e.graph.StateAttributes[ancestor] {
			if attr.Name != name {
				continue
			}
			scope := attr.Scope
			if scope == nil {
				scope = e.graph.Scope
			}
			return data, scope, true
		}
	}
	return nil, nil, false
}

func (e *StateExecutor) declaresAttribute(name string) bool {
	for _, attr := range e.graph.Attributes {
		if attr.Name == name {
			return true
		}
	}
	return false
}

func (e *StateExecutor) assignAttribute(name string, value Value) error {
	if e.occurrence != nil {
		if err := e.occurrence.SetFeatureValue(e.ctx, name, value); err != nil {
			return fmt.Errorf("%w: write %s of object #%d: %w",
				ErrStatePerformanceOccurrence, name, e.occurrence.ID, err)
		}
		fv, err := e.occurrence.GetFeatureValue(e.ctx, name)
		if err != nil {
			return fmt.Errorf("%w: read %s of object #%d after write: %w",
				ErrStatePerformanceOccurrence, name, e.occurrence.ID, err)
		}
		value = fv.HeldValue()
	} else if err := e.ctx.checkNamedWrite(e.graph.Scope, "state machine "+symbolText(e.stateMachine), name, &value); err != nil {
		// No occurrence holds this feature, so its declaration is checked here
		// rather than by the write to that occurrence.
		return err
	}
	e.stateData[name] = value
	return nil
}

// evalStepOf evaluates one expression of a step — a guard, a change condition, a
// duration — in scope, in an activation of its own (see beginStep), with the
// machine's data and the attributes of the state the step leaves shadowing it.
func (e *StateExecutor) evalStepOf(owner ast.Node, node ast.Node, scope *symbols.Scope) (Value, error) {
	ec := NewEvalContextIn(e.ctx, scope, e.self)
	ec.inBehaviorBody = true
	ec.Push(e.stateData)
	if state, ok := owner.(*ast.StateNode); ok {
		for _, frame := range e.attrFramesFor(state) {
			ec.Push(frame)
		}
	}
	defer ec.beginStep()()
	return ec.Eval(node)
}

// StateVertexName returns the name of a StateNode or PseudostateNode, "" for
// any other node.
func StateVertexName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.StateNode:
		return n.Name
	case *ast.PseudostateNode:
		return n.Name
	case *ast.Usage:
		if lower.IsTerminateUsage(n) {
			name, _ := ast.EffectiveName(n)
			return name
		}
		return ""
	default:
		return ""
	}
}

// getParentChain returns all ancestor states from child to root (inclusive).
// Result is ordered: [child, parent, grandparent, ...]
func (e *StateExecutor) getParentChain(state *ast.StateNode) []*ast.StateNode {
	chain := []*ast.StateNode{state}
	current := state
	for {
		parent, hasParent := e.graph.ParentState[current]
		if !hasParent {
			break
		}
		chain = append(chain, parent)
		current = parent
	}
	return chain
}

// getLCA finds the lowest common ancestor of two states.
// Returns nil if states are in different hierarchies.
func (e *StateExecutor) getLCA(state1, state2 *ast.StateNode) *ast.StateNode {
	chain1 := e.getParentChain(state1)
	chain2 := e.getParentChain(state2)

	// Build set from chain1
	chain1Set := make(map[*ast.StateNode]bool)
	for _, s := range chain1 {
		chain1Set[s] = true
	}

	// Find first common ancestor in chain2
	for _, s := range chain2 {
		if chain1Set[s] {
			return s
		}
	}

	return nil // No common ancestor
}

// scheduleTransitionEvents schedules TimeEvents for the outgoing transitions of
// the active configuration, in region declaration order: the order the
// transitions are queued in is observable. A time trigger on a composite state
// counts from entering it, so the enclosing states of every active leaf are
// scheduled too, and their timers are left alone while they stay active.
func (e *StateExecutor) scheduleTransitionEvents() error {
	for _, leaf := range e.activeLeaves() {
		if err := e.scheduleFromLeaf(leaf); err != nil {
			return err
		}
	}
	return nil
}

// scheduleFromLeaf schedules the outgoing transitions of an active leaf and the
// time transitions of the composite states enclosing it.
func (e *StateExecutor) scheduleFromLeaf(leaf *ast.StateNode) error {
	if err := e.scheduleTransitionsForState(leaf); err != nil {
		return err
	}
	for _, ancestor := range e.getParentChain(leaf)[1:] {
		if err := e.scheduleTimeTransitions(ancestor); err != nil {
			return err
		}
	}
	return nil
}

// scheduleCompletionTransitions queues a state's completion as one event, carrying
// its first completion transition; the guards are read when the occurrence is
// dispatched (chooseCompletion), not now. A state completes only once its do
// behavior has finished, so a state still running one is skipped here and
// scheduled by settleDoActions when the behavior ends; a composite state's body
// reaching `done` schedules it through completeIfDone.
func (e *StateExecutor) scheduleCompletionTransitions(state *ast.StateNode) error {
	if e.hasRunningDoAction(state) {
		return nil
	}
	for _, trans := range e.graph.Transitions[state] {
		if trans.Trigger != nil {
			continue
		}
		e.eventQueue.Push(Event{
			ID:        e.nextEventID,
			Type:      EventTime, // Use EventTime with nil trigger
			Timestamp: e.ctx.clock.now,
			Payload:   trans,
		})
		e.nextEventID++
		return nil
	}
	return nil
}

// scheduleTransitionsForState schedules events for outgoing transitions of a specific state.
func (e *StateExecutor) scheduleTransitionsForState(state *ast.StateNode) error {
	if err := e.scheduleCompletionTransitions(state); err != nil {
		return err
	}
	return e.scheduleTimeTransitions(state)
}

// scheduleTimeTransitions queues a time event per time-triggered transition out
// of the state whose timer is not running yet, due at the clock's instant.
func (e *StateExecutor) scheduleTimeTransitions(state *ast.StateNode) error {
	for _, trans := range e.graph.Transitions[state] {
		if e.timerScheduled[trans] {
			continue
		}
		if trans.Trigger == nil {
			continue // a completion transition, scheduled once the do behavior ends
		} else if timeEvent, ok := trans.Trigger.(*ast.TimeEvent); ok {
			if err := e.checkTimeTriggerType(trans, timeEvent); err != nil {
				return err
			}
			// Evaluate duration expression in the scope the transition was written
			// in, the machine's data shadowing it.
			durationVal, err := e.evalStepOf(trans.Source, timeEvent.Duration, trans.Scope)
			if err != nil {
				return fmt.Errorf("eval time duration: %w", err)
			}
			due, err := e.ctx.dueInstant(timeEvent, durationVal, "time duration")
			if err != nil {
				return err
			}

			e.eventQueue.Push(Event{
				ID:        e.nextEventID,
				Type:      EventTime,
				Timestamp: due,
				Payload:   trans,
			})
			e.nextEventID++
			e.timerScheduled[trans] = true
		}
	}

	return nil
}

// checkTimeTriggerType refuses, before evaluating it, the trigger argument
// validation refuses. The verdict is static, so it is judged once per transition.
func (e *StateExecutor) checkTimeTriggerType(trans *lower.Transition, t *ast.TimeEvent) error {
	if err, ok := e.timeTriggerVerdict[trans]; ok {
		return err
	}
	err := e.ctx.judgeTimeTriggerType(trans.Scope, t)
	e.timeTriggerVerdict[trans] = err
	return err
}

// processNextEvent pops and processes the next event from queue. It is one
// run-to-completion step: the event is dispatched, and only once it has been
// fully handled are events the new configuration no longer defers dispatched
// again.
func (e *StateExecutor) processNextEvent() error {
	if e.eventQueue.Len() == 0 {
		return fmt.Errorf("no events to process")
	}

	event, err := e.nextEvent()
	if err != nil {
		return err
	}
	e.moved = true
	// The clock never lags a dispatched event: a timer popped ahead of it moves it.
	e.ctx.clock.now = math.Max(e.ctx.clock.now, event.Timestamp)
	e.lastEventAt = e.ctx.clock.now

	e.markDispatch()
	dispatch, err := e.dispatchEvent(event)
	if err != nil {
		return err
	}
	e.lastDispatch = &dispatch
	e.recallDeferredEvents()
	e.pauseAtBreakpoint()
	return nil
}

// markDispatch opens a dispatch's account: where its fired transitions begin,
// with no state hit left staged by a dispatch that failed before pausing.
func (e *StateExecutor) markDispatch() {
	e.breakpointHit, e.dispatchMark = nil, -1
	if !e.keepFired {
		e.releaseFired(len(e.fired))
	}
	e.dispatchMark = len(e.fired)
}

// stagedBreakpoint is the breakpoint vertex the dispatch under way passed or
// entered, nil when none or no dispatch is under way. A pseudostate passed is
// read from the transitions the dispatch fired, so a failed firing stages none.
func (e *StateExecutor) stagedBreakpoint() ast.Node {
	if e.dispatchMark < 0 {
		return nil
	}
	for _, fired := range e.fired[min(e.dispatchMark, len(e.fired)):] {
		if _, ok := fired.Target.(*ast.PseudostateNode); ok && e.breakpointNodes[fired.Target] {
			return fired.Target
		}
	}
	if e.breakpointHit != nil {
		return e.breakpointHit
	}
	return nil
}

// pauseAtBreakpoint closes the dispatch just done, suspending the machine at the
// breakpoint vertex it passed or entered and leaving whatever is still due —
// the machine's own completion included — to the next run.
func (e *StateExecutor) pauseAtBreakpoint() {
	hit := e.stagedBreakpoint()
	e.breakpointHit, e.dispatchMark = nil, -1
	if hit == nil || e.state != StateRunning {
		return
	}
	e.pausedAt = hit
	e.state = StateSuspended
}

// nextEvent takes the event to dispatch off the queue: the earliest, unless the
// queue leaves several unordered at its head, when the policy draws which goes
// first and the draw is reported; a replay or check refusing the draw takes none.
func (e *StateExecutor) nextEvent() (Event, error) {
	tied := e.eventQueue.Tied()
	if len(tied) < 2 {
		return e.eventQueue.Pop(), nil
	}
	choice := ChoicePoint{
		Kind:         ChoiceDispatchOrder,
		Where:        dispatchWhere(tied[0].Timestamp),
		Alternatives: make([]string, len(tied)),
		File:         e.stateMachine.DocName,
		Span:         e.stateMachine.DeclSpan,
	}
	for i := range tied {
		choice.Alternatives[i] = e.eventLabel(tied[i])
	}
	scheduling := e.ctx.scheduling()
	choice.Taken = scheduling.choose(choice, nil)
	if err := scheduling.refusal(); err != nil {
		return Event{}, err
	}
	e.ctx.noteChoice(choice)
	event, _ := e.eventQueue.Take(tied[choice.Taken].ID)
	return event, nil
}

// dispatchWherePrefix opens where a dispatch order names its instant.
const dispatchWherePrefix = "events at t="

// dispatchWhere names the instant a dispatch order was drawn at.
func dispatchWhere(at float64) string {
	return dispatchWherePrefix + semantics.FormatReal(at)
}

// eventLabel names a queued event as a dispatch-order choice lists it: a time
// trigger by its state and the transition's declared position and target, as a
// transition choice names one; a pool event by what it accepts.
func (e *StateExecutor) eventLabel(event Event) string {
	if trans, ok := event.Payload.(*lower.Transition); ok && event.Type == EventTime {
		transitions := e.graph.Transitions[trans.Source]
		if pos := slices.Index(transitions, trans); pos >= 0 {
			return fmt.Sprintf("time %s %s", StateVertexName(trans.Source), transitionName(transitions, pos))
		}
		return transitionDescription(trans)
	}
	return eventName(&event)
}

// Dispatch is what became of an event a step took off the queue: a transition
// fired on it, a state deferred it, or nothing was enabled for it and it was
// dropped; Resumed names the states whose do behavior went on from an accept with it.
type Dispatch struct {
	Event    Event
	Fired    bool
	Deferred bool
	Resumed  []string
}

// LastDispatch returns what became of the event the last ProcessNextEvent
// dispatched, false when it dispatched none.
func (e *StateExecutor) LastDispatch() (Dispatch, bool) {
	if e.lastDispatch == nil {
		return Dispatch{}, false
	}
	return *e.lastDispatch, true
}

// FiredTransition is one transition taken: where it was written and the vertices
// it joined. An entry transition has no Source; it leaves the start of Owner's
// body, the one keying it in the graph's EntryTransitions (nil: the machine's own).
type FiredTransition struct {
	Decl   ast.Node
	Source ast.Node
	Target ast.Node
	Owner  ast.Node
}

// KeepFired has the machine keep its firings until FiredSince releases them, as
// a debugger reads them; off (the default), a dispatch keeps only its own.
func (e *StateExecutor) KeepFired(keep bool) {
	e.keepFired = keep
	if !keep {
		e.releaseFired(len(e.fired))
	}
}

// FiredTransitions returns the transitions kept, in firing order: compound
// transitions by segment, fork and join branches, and entry transitions.
func (e *StateExecutor) FiredTransitions() []FiredTransition {
	return slices.Clone(e.fired)
}

// FiredCount counts the transitions taken so far, released ones included: a mark
// to read what a later step fired from.
func (e *StateExecutor) FiredCount() int { return e.firedBase + len(e.fired) }

// FiredSince returns the kept transitions taken since mark, a FiredCount read
// earlier, and releases those before it: the machine keeps only from the last mark.
func (e *StateExecutor) FiredSince(mark int) []FiredTransition {
	e.releaseFired(mark - e.firedBase)
	if len(e.fired) == 0 {
		return nil
	}
	return slices.Clone(e.fired)
}

// releaseFired forgets the oldest n kept transitions, none of the dispatch under
// way, still counting them in FiredCount.
func (e *StateExecutor) releaseFired(n int) {
	n = min(n, len(e.fired))
	if e.dispatchMark >= 0 {
		n = min(n, e.dispatchMark)
	}
	if n <= 0 {
		return
	}
	kept := copy(e.fired, e.fired[n:])
	clear(e.fired[kept:])
	e.fired, e.firedBase = e.fired[:kept], e.firedBase+n
	if e.dispatchMark >= 0 {
		e.dispatchMark -= n
	}
}

// noteFired records transitions taken, skipping any without a declaration.
func (e *StateExecutor) noteFired(transitions ...*lower.Transition) {
	for _, trans := range transitions {
		if trans != nil && trans.Decl != nil {
			e.fired = append(e.fired, FiredTransition{Decl: trans.Decl, Source: trans.Source, Target: trans.Target})
		}
	}
}

// unfireOnError drops the transitions logged since mark when *err is set: a
// firing that failed midway took none of them.
func (e *StateExecutor) unfireOnError(mark int, err *error) {
	if *err != nil {
		e.fired = e.fired[:mark]
	}
}

// dispatchEvent delivers one event to the active configuration and reports what
// became of it.
func (e *StateExecutor) dispatchEvent(event Event) (Dispatch, error) {
	dispatch := Dispatch{Event: event}
	switch event.Type {
	case EventTime:
		// Fire transition - handle both old (TransitionEdge) and new (lower.Transition) for backward compatibility
		if lowerTrans, ok := event.Payload.(*lower.Transition); ok {
			// The timer has expired, so it is no longer running: a transition that
			// does not leave its source state re-arms it for the next round.
			delete(e.timerScheduled, lowerTrans)
			sourceState, _ := lowerTrans.Source.(*ast.StateNode)
			if sourceState != nil && !e.inActiveConfiguration(sourceState) {
				// The source was left before this event came up, so the transition
				// it carries is stale: firing it would move a state machine that is
				// no longer there.
				return dispatch, nil
			}
			var notes []RunNote
			if lowerTrans.Trigger == nil && sourceState != nil {
				var err error
				if lowerTrans, notes, err = e.chooseCompletion(sourceState, lowerTrans); err != nil || lowerTrans == nil {
					return dispatch, err
				}
			} else if lowerTrans.Trigger != nil {
				// The timer selects its transition as a signal dispatch would: its
				// guard holds and the join it may lead into is ready, or it fires nothing.
				if enabled, err := e.transitionEnabled(lowerTrans, &event); err != nil || !enabled {
					return dispatch, err
				}
			}
			// A transition out of a state inside an orthogonal region is region-local:
			// it must not tear down the sibling regions unless its target lies outside
			// the region set. The source may be a composite state enclosing the
			// region's active state, so the region is resolved by containment.
			var err error
			dispatch.Fired, err = e.firingOn(&event, func() (bool, error) {
				return e.resolveAndFire(sourceState, lowerTrans, notes)
			})
			return dispatch, err
		}

		// Fallback for tests that use TransitionEdge directly
		if edge, ok := event.Payload.(*ast.TransitionEdge); ok {
			// Convert TransitionEdge to lower.Transition
			// Need to find source/target states by name
			var sourceState, targetState *ast.StateNode
			for _, state := range e.graph.States {
				if edge.Source != nil && len(edge.Source.Parts) > 0 {
					if state.Name == edge.Source.Parts[len(edge.Source.Parts)-1].Text {
						sourceState = state
					}
				}
				if edge.Target != nil && len(edge.Target.Parts) > 0 {
					if state.Name == edge.Target.Parts[len(edge.Target.Parts)-1].Text {
						targetState = state
					}
				}
			}
			if sourceState == nil || targetState == nil {
				return dispatch, fmt.Errorf("could not find source/target states for transition")
			}

			lowerTrans := &lower.Transition{
				Source:  sourceState,
				Target:  targetState,
				Trigger: edge.Trigger,
				Guard:   edge.Guard,
				Effect:  lower.LowerBehaviors(edge.Effect, e.stateMachine.Scope, e.ctx.Resolver()),
			}
			var err error
			dispatch.Fired, err = e.fireTransition(lowerTrans, route{segments: []*lower.Transition{lowerTrans}, target: targetState})
			return dispatch, err
		}

		return dispatch, fmt.Errorf("invalid TimeEvent payload: expected *lower.Transition or *ast.TransitionEdge")
	default:
		// For general events, broadcast to all active regions
		consumed, resumed, err := e.broadcastEvent(&event)
		if err != nil {
			return dispatch, err
		}
		dispatch.Fired, dispatch.Resumed = consumed, resumed
		if !consumed && len(resumed) == 0 && e.defersEvent(&event) {
			dispatch.Deferred = true
			e.deferred = append(e.deferred, event)
		}
		return dispatch, nil
	}
}

// defersEvent reports whether any state of the active configuration, or an
// ancestor of one, defers this event. A composite state's deferral holds while
// any of its substates is active.
func (e *StateExecutor) defersEvent(event *Event) bool {
	return len(e.deferringStates(event)) > 0
}

// deferringStates lists the states of the active configuration, each active leaf
// and its ancestors, that defer this event; each is listed once.
func (e *StateExecutor) deferringStates(event *Event) []*ast.StateNode {
	var deferring []*ast.StateNode
	asked := make(map[*ast.StateNode]bool)
	for _, state := range e.activeStates() {
		for _, ancestor := range e.getParentChain(state) {
			if asked[ancestor] {
				continue
			}
			asked[ancestor] = true
			for _, trigger := range e.graph.Deferred[ancestor] {
				if e.triggerMatches(trigger, e.graph.StateScopes[ancestor], event) {
					deferring = append(deferring, ancestor)
					break
				}
			}
		}
	}
	return deferring
}

// deferralOutranks reports whether a deferring state of the configuration holds
// the event back from the selected transitions: only a transition out of that
// state, or out of a state nested in it, is nested deeply enough to override its
// deferral, and every deferring state must be overridden for any of them to fire.
func (e *StateExecutor) deferralOutranks(candidates []dispatchCandidate, event *Event) bool {
	for _, deferring := range e.deferringStates(event) {
		overridden := slices.ContainsFunc(candidates, func(candidate dispatchCandidate) bool {
			return e.encloses(deferring, candidate.source)
		})
		if !overridden {
			return true
		}
	}
	return false
}

// recallDeferredEvents returns every deferred event the configuration reached by
// the step just finished no longer defers to the event pool. A recalled event
// keeps its ID, so it is dispatched ahead of whatever arrived while it was held
// back, but not its original timestamp, which would move virtual time backwards.
func (e *StateExecutor) recallDeferredEvents() {
	if len(e.deferred) == 0 {
		return
	}
	retained := make([]Event, 0, len(e.deferred))
	for _, event := range e.deferred {
		if e.defersEvent(&event) {
			retained = append(retained, event)
			continue
		}
		event.Timestamp = e.ctx.clock.now
		e.eventQueue.Push(event)
	}
	e.deferred = retained
}

// broadcastEvent offers an event to the active configuration, reporting whether
// any transition consumed it and the states whose do behavior went on with it.
// An event nothing consumed is either deferred or dropped by the caller, so "a
// transition fired" and "nothing happened" must not look alike here.
//
// Dispatch selects the transitions to take against the configuration and data the
// event was taken off the queue for, so a state this event entered never reacts to
// it; the do behaviors parked at an accept for it then go on with it, and the
// selected transitions fire one at a time in the order the scheduling policy draws.
func (e *StateExecutor) broadcastEvent(event *Event) (bool, []string, error) {
	selected, err := e.selectTransitions(event)
	if err != nil {
		return false, nil, err
	}
	candidates, err := e.chooseTransitions(selected, event)
	if err != nil {
		return false, nil, err
	}
	// A witness move refused while drawing must stop the dispatch before the do
	// behaviors take the occurrence, so a refused replay changes nothing.
	if err := e.ctx.scheduling().refusal(); err != nil {
		return false, nil, err
	}
	var resumed []string
	if msg, ok := event.Payload.(Message); ok {
		taking, err := e.doBehaviorsTaking(msg, candidates)
		if err != nil {
			return false, nil, err
		}
		if resumed, err = e.resumeDoBehaviors(taking, msg); err != nil {
			return false, resumed, err
		}
	}
	consumed, err := e.dispatchInOrder("on "+eventName(event), candidates, func(candidate dispatchCandidate, trans *lower.Transition, notes []RunNote) (bool, error) {
		// The guard ran against the pre-dispatch data, so the arguments it read were
		// unbound again; the effect needs them bound.
		unbind, err := e.bindTriggerArguments(trans, event)
		if err != nil {
			unbind()
			return false, fmt.Errorf("state %s: %w", candidate.source.Name, err)
		}
		fired, err := e.firingOn(event, func() (bool, error) {
			return e.fireFrom(candidate.source, trans, notes, candidate.route)
		})
		if err != nil {
			return false, fmt.Errorf("fire transition out of %s: %w", candidate.source.Name, err)
		}
		return fired, nil
	})
	return consumed, resumed, err
}

// firingOn runs fire with event as the occurrence the transition taken reacts to.
func (e *StateExecutor) firingOn(event *Event, fire func() (bool, error)) (bool, error) {
	saved := e.firingEvent
	e.firingEvent = event
	defer func() { e.firingEvent = saved }()
	return fire()
}

// dispatchInOrder fires the chosen candidates one at a time through fire, the
// policy re-drawing among those still active since a reaction may leave a leaf;
// where names the occurrence dispatched, for the choice each draw reports.
func (e *StateExecutor) dispatchInOrder(
	where string,
	candidates []dispatchCandidate,
	fire func(dispatchCandidate, *lower.Transition, []RunNote) (bool, error),
) (bool, error) {
	pending := slices.Clone(candidates)
	acted := false
	for len(pending) > 0 {
		pending = slices.DeleteFunc(pending, func(c dispatchCandidate) bool { return !e.isActive(c.leaf) })
		if len(pending) == 0 {
			break
		}
		next, order := e.chooseRegion(where, pending)
		candidate := pending[next]
		pending = slices.Delete(pending, next, next+1)
		notes := candidate.notes
		if order != nil {
			notes = append([]RunNote{order}, notes...)
		}
		if err := e.ctx.scheduling().refusal(); err != nil {
			return acted, err
		}
		fired, err := fire(candidate, candidate.chosen, notes)
		acted = acted || fired
		if err != nil {
			return acted, err
		}
		if e.state.Ended() {
			break
		}
	}
	return acted, nil
}

// dispatchCandidate is the state one active leaf selected for an event, the leaf
// itself or a composite state enclosing it, with the positions of the transitions
// out of it the event enables. Which of them fires, and the state it is routed to,
// is settled only once the candidate survives conflict resolution
// (chooseTransitions); what selecting and choosing found worth noting is recorded
// only if it fires.
type dispatchCandidate struct {
	leaf    *ast.StateNode
	source  *ast.StateNode
	enabled []int
	notes   []RunNote
	chosen  *lower.Transition
	route   route
}

// selectTransitions picks one source state per leaf active when the event is
// dispatched: a transition out of a composite state is enabled while any of its
// substates is active, so the walk goes outward from the leaf and stops at the
// innermost state with an enabled transition. A false guard does not consume
// the event, so the walk carries on past it. Leaves in sibling regions of one
// composite state select the same state, which the event still leaves only once.
// A state that defers the event outranks every transition not nested in it: it
// selects nothing, to defer the event, unless each deferring state is overridden.
func (e *StateExecutor) selectTransitions(event *Event) ([]dispatchCandidate, error) {
	candidates, err := e.selectCandidates(func(source *ast.StateNode) ([]int, []RunNote, error) {
		return e.enabledTransitions(source, event)
	})
	if err != nil {
		return nil, err
	}
	if e.deferralOutranks(candidates, event) {
		return nil, nil
	}
	return candidates, nil
}

// selectCandidates walks outward from every active leaf, asking enabled which
// transitions that state offers and what finding them noted, and collects one
// candidate per leaf. A state is asked once per dispatch, however many leaves
// reach it.
func (e *StateExecutor) selectCandidates(
	enabled func(*ast.StateNode) ([]int, []RunNote, error),
) ([]dispatchCandidate, error) {
	var candidates []dispatchCandidate
	offered := make(map[*ast.StateNode][]int)
	for _, leaf := range e.activeLeaves() {
		for _, source := range e.getParentChain(leaf) {
			if positions, asked := offered[source]; asked {
				if len(positions) > 0 {
					break
				}
				continue
			}
			positions, notes, err := enabled(source)
			if err != nil {
				return nil, fmt.Errorf("state %s: %w", source.Name, err)
			}
			offered[source] = positions
			if len(positions) == 0 {
				continue
			}
			candidates = append(candidates, dispatchCandidate{leaf: leaf, source: source, enabled: positions, notes: notes})
			break
		}
	}
	return candidates, nil
}

// chooseTransitions resolves the dispatch: the candidates not outranked by a nested
// one, each with the one of its enabled transitions that fires drawn once here and
// its route through any junctions read against the pre-dispatch data, for the do
// behaviors taking the occurrence, the firing and the preview alike; a junction
// several branches of which hold is drawn among only as the candidate fires. A
// state outranked by a nested one draws nothing. event is nil for a change poll.
func (e *StateExecutor) chooseTransitions(candidates []dispatchCandidate, event *Event) ([]dispatchCandidate, error) {
	chosen := make([]dispatchCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if e.losesToNestedTransition(candidates, candidate) {
			continue
		}
		candidate.chosen, candidate.notes = e.chooseTransition(candidate)
		route, err := e.resolveRouteFor(candidate.chosen, event)
		if err != nil {
			return nil, fmt.Errorf("transition out of %s: %w", candidate.source.Name, err)
		}
		candidate.route = route
		chosen = append(chosen, candidate)
	}
	return chosen, nil
}

// resolveRouteFor resolves trans's route with the trigger's arguments bound, so a
// pseudostate guard along it reads them as the transition's own guard does.
func (e *StateExecutor) resolveRouteFor(trans *lower.Transition, event *Event) (route, error) {
	if event == nil {
		return e.resolveRoute(trans)
	}
	unbind, err := e.bindTriggerArguments(trans, event)
	defer unbind()
	if err != nil {
		return route{}, err
	}
	return e.resolveRoute(trans)
}

// chooseTransition resolves which of the candidate's enabled transitions fires,
// with the choice point it makes ahead of the candidate's notes.
func (e *StateExecutor) chooseTransition(candidate dispatchCandidate) (*lower.Transition, []RunNote) {
	transitions := e.graph.Transitions[candidate.source]
	notes := candidate.notes
	pick := 0
	if choice, ok := e.transitionChoice(candidate.source, transitions, candidate.enabled); ok {
		whereOf := func(i int) string { return transitionWhere(candidate.source, transitions[candidate.enabled[i]]) }
		pick = e.ctx.scheduling().choose(choice, whereOf)
		choice.Taken, choice.Where = pick, whereOf(pick)
		choice.File, choice.Span = e.transitionLocation(candidate.source, transitions[candidate.enabled[pick]])
		notes = append([]RunNote{choice}, notes...)
	}
	return transitions[candidate.enabled[pick]], notes
}

// chooseRegion resolves which of the candidates, all still able to fire, fires next:
// the policy draws the pick and, with several candidates, the choice is reported.
func (e *StateExecutor) chooseRegion(where string, pending []dispatchCandidate) (int, RunNote) {
	if len(pending) < 2 {
		return 0, nil
	}
	sources := make([]*ast.StateNode, len(pending))
	for i, candidate := range pending {
		sources[i] = candidate.source
	}
	choice := e.regionOrderChoice(where, sources)
	return choice.Taken, choice
}

// regionOrderChoice is the choice among the states of several regions acting on
// one occasion, the one the policy picks first.
func (e *StateExecutor) regionOrderChoice(where string, states []*ast.StateNode) ChoicePoint {
	choice := ChoicePoint{
		Kind:         ChoiceRegionOrder,
		Where:        where,
		Alternatives: e.stateNames(states),
		File:         e.stateMachine.DocName,
	}
	choice.Taken = e.ctx.scheduling().choose(choice, nil)
	choice.Span = states[choice.Taken].Span()
	return choice
}

// stateNames spells each state, qualified by its region's name where two of the
// states share a name, as typed regions' states do.
func (e *StateExecutor) stateNames(states []*ast.StateNode) []string {
	shared := make(map[string]int, len(states))
	for _, state := range states {
		shared[state.Name]++
	}
	names := make([]string, len(states))
	for i, state := range states {
		names[i] = state.Name
		if region := e.graph.RegionOf[state]; shared[names[i]] > 1 && region != nil && region.Name != "" {
			names[i] = region.Name + "." + names[i]
		}
	}
	return names
}

// losesToNestedTransition reports whether another leaf selected a transition out
// of a state nested inside this candidate's source: two transitions leaving the
// same state are in conflict, and the innermost one wins.
func (e *StateExecutor) losesToNestedTransition(candidates []dispatchCandidate, candidate dispatchCandidate) bool {
	for _, other := range candidates {
		if other.source != candidate.source && e.nestedIn(other.source, candidate.source) {
			return true
		}
	}
	return false
}

// nestedIn reports whether state lies inside the given composite state.
func (e *StateExecutor) nestedIn(state, composite *ast.StateNode) bool {
	for _, ancestor := range e.getParentChain(state)[1:] {
		if ancestor == composite {
			return true
		}
	}
	return false
}

// encloses reports whether the source is the target or contains it, making the
// transition external: KerML exits the source of every transition, so a state
// transitioning to itself is left and entered afresh.
func (e *StateExecutor) encloses(source, target *ast.StateNode) bool {
	if source == nil || target == nil {
		return false
	}
	return source == target || e.nestedIn(target, source)
}

// exitStates exits the states being left, innermost first.
func (e *StateExecutor) exitStates(leaving []*ast.StateNode) error {
	for _, state := range leaving {
		if e.exitedByAncestorRegion(state, leaving) {
			continue
		}
		if err := e.exitState(state); err != nil {
			return fmt.Errorf("exit state: %w", err)
		}
	}
	return nil
}

// exitedByAncestorRegion reports whether another state being left owns the region
// the state is active in, and so exits it recursively.
func (e *StateExecutor) exitedByAncestorRegion(state *ast.StateNode, leaving []*ast.StateNode) bool {
	region := e.graph.RegionOf[state]
	if region == nil {
		return false
	}
	owner := e.graph.RegionOwner[region]
	for _, other := range leaving {
		if other == owner {
			return true
		}
	}
	return false
}

// activeLeaves returns the innermost active states, ordered by the declaration of
// the regions they lie in rather than by their depth. A state owning an active
// orthogonal region is not a leaf: the event reaches it walking outward. Once
// every one of its regions rests at the state itself, it is the leaf, once.
func (e *StateExecutor) activeLeaves() []*ast.StateNode {
	states := e.activeStates()
	leaves := make([]*ast.StateNode, 0, len(states))
	seen := make(map[*ast.StateNode]bool, len(states))
	for _, state := range states {
		if !e.enclosesActiveRegion(state) && !seen[state] {
			seen[state] = true
			leaves = append(leaves, state)
		}
	}
	paths := make(map[*ast.StateNode][]int, len(leaves))
	for _, leaf := range leaves {
		paths[leaf] = e.regionPath(leaf)
	}
	sort.SliceStable(leaves, func(i, j int) bool {
		return lessPath(paths[leaves[i]], paths[leaves[j]])
	})
	return leaves
}

// regionPath returns the declaration index of every region between the machine
// and the state, outermost first, which orders concurrent states.
func (e *StateExecutor) regionPath(state *ast.StateNode) []int {
	chain := e.getParentChain(state)
	path := make([]int, 0, len(chain))
	for i := len(chain) - 1; i >= 0; i-- {
		region := e.graph.RegionOf[chain[i]]
		if region == nil {
			continue
		}
		siblings := e.graph.TopRegions
		if owner := e.graph.RegionOwner[region]; owner != nil {
			siblings = e.graph.CompositeStates[owner]
		}
		for index, sibling := range siblings {
			if sibling == region {
				path = append(path, index)
				break
			}
		}
	}
	return path
}

// lessPath orders two region paths lexicographically, a shorter path first where
// one prefixes the other.
func lessPath(a, b []int) bool {
	for i := range a {
		if i >= len(b) {
			return false
		}
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

// fireFrom takes a transition whose source is the given active state, which is
// either the active leaf or a composite state enclosing it. A source lying in an
// active orthogonal region moves that region; one outside every active region
// moves the machine's single active hierarchy. notes are recorded only if it fires;
// r is the transition's route as resolveRoute settled it.
func (e *StateExecutor) fireFrom(source *ast.StateNode, trans *lower.Transition, notes []RunNote, r route) (bool, error) {
	saved := e.firingNotes
	e.firingNotes = notes
	defer func() { e.firingNotes = saved }()
	if region := e.activeRegionOf(source); region != nil {
		return e.fireTransitionInRegion(region, trans, r)
	}
	return e.fireTransition(trans, r)
}

// resolveAndFire takes a transition outside a dispatch, a timer come due, resolving
// its route as it fires; source is as for fireFrom, nil for the single hierarchy.
func (e *StateExecutor) resolveAndFire(source *ast.StateNode, trans *lower.Transition, notes []RunNote) (bool, error) {
	r, err := e.resolveRoute(trans)
	if err != nil {
		return false, err
	}
	if source != nil {
		return e.fireFrom(source, trans, notes, r)
	}
	saved := e.firingNotes
	e.firingNotes = notes
	defer func() { e.firingNotes = saved }()
	return e.fireTransition(trans, r)
}

// chooseCompletion resolves which completion transition out of source fires on
// the completion event dispatched carries: every completion transition of the
// state has its guard read now, the policy draws one of those enabled as a
// transition choice and the state's other completion events leave the queue,
// one completion occurrence firing one transition. None enabled fires nothing.
func (e *StateExecutor) chooseCompletion(source *ast.StateNode, dispatched *lower.Transition) (*lower.Transition, []RunNote, error) {
	queued := e.eventQueue.CompletionsOf(source)
	// The others leave the queue only once the draw stands: a refused replay changes nothing.
	drain := func() {
		for _, ev := range queued {
			e.eventQueue.Take(ev.ID)
		}
	}
	transitions := e.graph.Transitions[source]
	if completionCount(transitions) < 2 {
		// Nothing to choose among: firing reads the one guard.
		drain()
		return dispatched, nil, nil
	}
	var enabled []int
	var notes []RunNote
	for pos, trans := range transitions {
		if trans.Trigger != nil {
			continue
		}
		var ok bool
		var err error
		if len(enabled) > 0 {
			// As for a triggered event: once one is enabled, a later one whose guard
			// cannot be read is noted as an alternative not taken, not an error.
			e.preview(func() { ok, err = e.completionEnabled(trans) })
			if err != nil {
				notes = append(notes, e.unevaluableTransition(source, transitions, pos, err))
				ok = false
			}
		} else if ok, err = e.completionEnabled(trans); err != nil {
			return nil, nil, fmt.Errorf("eval completion guard: %w", err)
		}
		if ok {
			enabled = append(enabled, pos)
		}
	}
	if len(enabled) == 0 {
		drain()
		return nil, nil, nil
	}
	choice, ok := e.transitionChoice(source, transitions, enabled)
	if !ok {
		drain()
		return transitions[enabled[0]], notes, nil
	}
	whereOf := func(i int) string { return transitionWhere(source, transitions[enabled[i]]) }
	pick := e.ctx.scheduling().choose(choice, whereOf)
	choice.Taken, choice.Where = pick, whereOf(pick)
	choice.File, choice.Span = e.transitionLocation(source, transitions[enabled[pick]])
	if err := e.ctx.scheduling().refusal(); err != nil {
		return nil, nil, err
	}
	drain()
	return transitions[enabled[pick]], append(notes, choice), nil
}

// completionCount is how many of the transitions are completion transitions.
func completionCount(transitions []*lower.Transition) int {
	n := 0
	for _, trans := range transitions {
		if trans.Trigger == nil {
			n++
		}
	}
	return n
}

// completionEnabled reports whether a completion transition can fire now: its
// guard holds and the join it may lead into has every other branch in place.
func (e *StateExecutor) completionEnabled(trans *lower.Transition) (bool, error) {
	pass, err := e.passesGuard(trans)
	if err != nil || !pass {
		return false, err
	}
	return e.joinSynchronized(trans, nil)
}

// transitionDecided records what selecting the transition now firing noted, its
// guard having passed its final reading, then what settling its route r noted;
// the route is returned with its notes taken.
func (e *StateExecutor) transitionDecided(r route) route {
	e.ctx.noteAll(e.firingNotes)
	e.firingNotes = nil
	e.ctx.noteAll(r.notes)
	r.notes = nil
	return r
}

// activeRegionOf returns the innermost active orthogonal region the state is
// declared in, or nil when it lies outside every active region.
func (e *StateExecutor) activeRegionOf(state *ast.StateNode) *ast.StateRegion {
	for _, ancestor := range e.getParentChain(state) {
		region, inRegion := e.graph.RegionOf[ancestor]
		if !inRegion {
			continue
		}
		if _, active := e.activeConfig.regionStates[region]; active {
			return region
		}
	}
	return nil
}

// enclosesActiveRegion reports whether the state owns an orthogonal region with
// an active state below it; a region resting at the state itself has none.
func (e *StateExecutor) enclosesActiveRegion(state *ast.StateNode) bool {
	for _, region := range e.graph.CompositeStates[state] {
		if active, ok := e.activeConfig.regionStates[region]; ok && active != state {
			return true
		}
	}
	return false
}

// enabledTransitions returns the positions of the transitions out of state that
// this event triggers and whose guards hold, none when the state cannot react to
// it. A transition whose guard is false does not consume the event, so a later
// one still gets its chance. Selection leaves the machine's data as it was: the
// caller binds the trigger's arguments again before firing. Every transition is
// examined so that several enabled at once are a choice point; the notes are
// the caller's to record if a transition fires.
func (e *StateExecutor) enabledTransitions(state *ast.StateNode, event *Event) ([]int, []RunNote, error) {
	var enabled []int
	var notes []RunNote
	transitions := e.graph.Transitions[state]
	// Once one is enabled the transition is decided; the rest are probed only to
	// report the choice, which leaves the run as it was.
	for i, trans := range transitions {
		var ok bool
		if len(enabled) > 0 {
			var unevaluable *UnevaluableGuard
			if ok, unevaluable = e.probeTransition(state, transitions, i, event); unevaluable != nil {
				notes = append(notes, *unevaluable)
			}
		} else {
			var err error
			if ok, err = e.transitionEnabled(trans, event); err != nil {
				return nil, nil, err
			}
		}
		if ok {
			enabled = append(enabled, i)
		}
	}
	return enabled, notes, nil
}

// probeTransition reads whether the transition at position i out of state reacts
// to event once another already does, as a probe the context undoes whole. One
// that cannot be evaluated is not selected and is returned as the note to record.
func (e *StateExecutor) probeTransition(state *ast.StateNode, transitions []*lower.Transition, i int, event *Event) (bool, *UnevaluableGuard) {
	var ok bool
	var err error
	e.preview(func() { ok, err = e.transitionEnabled(transitions[i], event) })
	if err != nil {
		note := e.unevaluableTransition(state, transitions, i, err)
		return false, &note
	}
	return ok, nil
}

// transitionEnabled reports whether trans reacts to event: its trigger matches,
// its guard holds and the join it may lead into is ready to fire.
func (e *StateExecutor) transitionEnabled(trans *lower.Transition, event *Event) (bool, error) {
	matches, err := e.matchesEvent(trans, event)
	if err != nil || !matches {
		return false, err
	}
	// A call trigger's arguments are bound before the guard runs: the guard is
	// written against the parameters the trigger declares. A transition that
	// does not fire must leave no trace of them in the machine's data.
	unbind, err := e.bindTriggerArguments(trans, event)
	if err != nil {
		unbind()
		return false, err
	}
	pass, err := e.passesGuard(trans)
	unbind()
	if err != nil || !pass {
		return false, err
	}
	// A transition into a join whose other branches have not arrived is not
	// enabled either: firing it would move nothing.
	return e.joinSynchronized(trans, event)
}

// transitionChoice is the transitions out of state enabled for one event, at
// their declared positions, as a choice point; there is none under two. pick is
// the position in enabled of the one that fires.
func (e *StateExecutor) transitionChoice(state *ast.StateNode, transitions []*lower.Transition, enabled []int) (ChoicePoint, bool) {
	if len(enabled) < 2 {
		return ChoicePoint{}, false
	}
	alts := make([]string, len(enabled))
	for i, pos := range enabled {
		alts[i] = transitionName(transitions, pos)
	}
	return ChoicePoint{Kind: ChoiceTransition, Alternatives: alts}, true
}

// unevaluableTransition is the transition at position pos out of state, probed
// once another was enabled, as the note that it cannot be evaluated.
func (e *StateExecutor) unevaluableTransition(state *ast.StateNode, transitions []*lower.Transition, pos int, err error) UnevaluableGuard {
	trans := transitions[pos]
	file, span := e.transitionLocation(state, trans)
	if trans.Guard != nil {
		span = trans.Guard.Span()
	}
	return UnevaluableGuard{
		Where:       transitionWhere(state, trans),
		Alternative: transitionName(transitions, pos),
		Reason:      err.Error(),
		File:        file,
		Span:        span,
	}
}

// transitionName names a transition out of a state by declared position and target.
func transitionName(transitions []*lower.Transition, pos int) string {
	return fmt.Sprintf("%d->%s", pos+1, StateVertexName(transitions[pos].Target))
}

// transitionWhere names the state and the event trans reacts to, for a note.
func transitionWhere(state *ast.StateNode, trans *lower.Transition) string {
	where := "state " + state.Name
	if name := triggerName(trans.Trigger); name != "" {
		where += " on " + name
	}
	return where
}

// transitionLocation is where trans was declared, or its vertex when it has no declaration.
func (e *StateExecutor) transitionLocation(vertex ast.Node, trans *lower.Transition) (string, source.Span) {
	file := e.stateMachine.DocName
	if trans.Scope != nil && trans.Scope.DocName() != "" {
		file = trans.Scope.DocName()
	}
	if trans.Decl != nil {
		return file, trans.Decl.Span()
	}
	return file, vertex.Span()
}

// bindTriggerArguments binds the parameters a call trigger declares to the
// arguments of the invocation, so the transition's guard and effect can read
// them. It returns the function restoring the machine's data to what it held
// before, for the caller to run when the transition does not fire.
func (e *StateExecutor) bindTriggerArguments(trans *lower.Transition, event *Event) (func(), error) {
	if acceptEvent, ok := trans.Trigger.(*ast.AcceptEvent); ok {
		return e.bindAcceptPayload(acceptEvent, event)
	}

	callEvent, ok := trans.Trigger.(*ast.CallEvent)
	if !ok || len(callEvent.Parameters) == 0 {
		return func() { /* nothing was bound */ }, nil
	}
	unbind := e.restoreData(callEvent.Parameters)
	call, ok := event.Payload.(Call)
	if !ok {
		return unbind, fmt.Errorf("call trigger %s: event carries %T, not an operation invocation",
			ast.SimpleName(callEvent.Operation), event.Payload)
	}
	for _, param := range callEvent.Parameters {
		value, ok := call.Args[param.Text]
		if !ok {
			return unbind, fmt.Errorf("call trigger %s: invocation carries no argument %q",
				call.Operation, param.Text)
		}
		e.stateData[param.Text] = value
	}
	return unbind, nil
}

// bindAcceptPayload binds the name an accept gave its payload
// (`accept msg : Warning`) to the value the accepted occurrence carries, for the
// transition's guard and effect to read, and returns the function unbinding it.
func (e *StateExecutor) bindAcceptPayload(acceptEvent *ast.AcceptEvent, event *Event) (func(), error) {
	if acceptEvent.Payload == nil || acceptEvent.Payload.Ident.Name == "" {
		return func() { /* nothing was bound */ }, nil
	}
	name := ast.NameSegment{Text: acceptEvent.Payload.Ident.Name}
	unbind := e.restoreData([]ast.NameSegment{name})
	msg, ok := event.Payload.(Message)
	if !ok {
		return unbind, fmt.Errorf("accept %s: event carries %T, not a message",
			name.Text, event.Payload)
	}
	value, err := e.ctx.acceptedValue(&msg)
	if err != nil {
		return unbind, fmt.Errorf("accept %s: %w", name.Text, err)
	}
	event.Payload = msg
	e.stateData[name.Text] = value
	return unbind, nil
}

// orAnonymousSignal names the signal a message carries for a diagnostic.
func orAnonymousSignal(signalType string) string {
	if signalType == "" {
		return "the accepted message"
	}
	return "the accepted " + signalType
}

// restoreData snapshots the named entries of the machine's data and returns the
// function putting them back, deleting the ones that were not there before.
func (e *StateExecutor) restoreData(names []ast.NameSegment) func() {
	saved := make(map[string]Value, len(names))
	held := make(map[string]bool, len(names))
	for _, name := range names {
		value, ok := e.stateData[name.Text]
		saved[name.Text], held[name.Text] = value, ok
	}
	return func() {
		for name, wasHeld := range held {
			if wasHeld {
				e.stateData[name] = saved[name]
			} else {
				delete(e.stateData, name)
			}
		}
	}
}

// matchesEvent checks if a transition matches the given event. Resolving the
// port a `via` names may materialize it, which can fail.
func (e *StateExecutor) matchesEvent(trans *lower.Transition, event *Event) (bool, error) {
	// Completion transition (nil trigger) doesn't match external events
	if trans.Trigger == nil {
		return false, nil
	}

	switch event.Type {
	case EventChange:
		// A change occurrence is the poll that observed the rise: it takes the
		// change-triggered transitions whose condition rose in it, not yet latched.
		poll, ok := event.Payload.(*changePoll)
		return ok && e.triggerMatches(trans.Trigger, trans.Scope, event) && poll.condition[trans] && !e.changeFired[trans], nil

	case EventAccept, EventCall:
		if !e.triggerMatches(trans.Trigger, trans.Scope, event) {
			return false, nil
		}
		// A transfer is taken by the trigger whose receiver it reaches: the port
		// a `via` names, or the performer itself when it names none.
		msg, ok := event.Payload.(Message)
		if !ok {
			return trans.Via == "", nil
		}
		return e.ctx.messageReaches(msg, e.stateMachine.Name, trans.Via, e.self)

	case EventTime:
		// Time events carry the specific transition in Payload
		// If matchesEvent is called for time events (shouldn't normally happen),
		// match if this transition is the one in the payload
		if transPayload, ok := event.Payload.(*lower.Transition); ok {
			return trans == transPayload, nil
		}
		return false, nil

	default:
		return false, nil
	}
}

// triggerMatches reports whether a trigger reacts to an event, whether the
// trigger belongs to a transition or to a state's deferred set. scope is where
// the trigger was declared, in which the type it accepts resolves.
func (e *StateExecutor) triggerMatches(trigger ast.Node, scope *symbols.Scope, event *Event) bool {
	switch event.Type {
	case EventAccept:
		acceptEvent, ok := trigger.(*ast.AcceptEvent)
		if !ok {
			return false
		}
		msg, ok := event.Payload.(Message)
		if !ok {
			return false
		}
		// The accept names the occurrence it takes either by its type
		// (`accept Ping`) or by the event it subsets (`accept :> shutDown`).
		if typed := ast.AsQualifiedName(acceptEvent.SignalType); typed != nil && len(typed.Parts) > 0 {
			return e.ctx.messageMatches(msg, typed, scope)
		}
		if lower.FeaturePath(acceptEvent.Subsets) == "" {
			return false
		}
		return e.triggerEval(scope).carriesEvent(msg, acceptEvent.Subsets)

	case EventCall:
		callEvent, ok := trigger.(*ast.CallEvent)
		if !ok {
			return false
		}
		call, ok := event.Payload.(Call)
		if !ok {
			return false
		}
		// A trigger naming an operation fires only for that operation; a trigger
		// naming none fires for any call.
		expectedOp := ast.SimpleName(callEvent.Operation)
		if expectedOp != "" && expectedOp != call.Operation {
			return false
		}
		// A trigger declaring parameters fires only for a call carrying an
		// argument of each declared name; `op()` takes the call whatever it carries.
		for _, param := range callEvent.Parameters {
			if _, ok := call.Args[param.Text]; !ok {
				return false
			}
		}
		return true

	case EventChange:
		// Re-evaluate condition (pollChangeEvents is the primary driver); here we
		// just verify it is a change trigger with a condition.
		changeEvent, ok := trigger.(*ast.ChangeEvent)
		return ok && changeEvent.Condition != nil

	default:
		return false
	}
}

// fireTransition takes a state transition, reporting whether it was taken: one
// whose guard is false leaves the machine where it is; one whose route is open
// at a junction draw has the draw made as the move begins.
func (e *StateExecutor) fireTransition(trans *lower.Transition, r route) (fired bool, err error) {
	defer e.unfireOnError(len(e.fired), &err)
	pass, err := e.passesGuard(trans)
	if err != nil || !pass {
		return false, err
	}
	// Fork, join and history reshape the active configuration rather than moving
	// to a single state, so they are fired whole; a join decides itself once ready.
	if ps, ok := trans.Target.(*ast.PseudostateNode); ok && isSynchronizationTarget(ps) {
		switch ps.Kind {
		case ast.PseudostateFork:
			e.transitionDecided(r)
			return true, e.fireForkTransition(trans, ps)
		case ast.PseudostateJoin:
			return e.fireJoinTransition(trans, ps, r)
		default:
			return true, e.fireHistoryTransition(trans, ps, e.transitionDecided(r))
		}
	}
	if !r.settled() {
		return false, fmt.Errorf("transition target state not found")
	}
	return true, e.transitionTo(trans, e.transitionDecided(r))
}

// moveOrigin is the state a move of the single active hierarchy starts from: the
// active simple state, or the composite state whose regions hold the configuration.
func (e *StateExecutor) moveOrigin() *ast.StateNode {
	if current := e.getCurrentState(); current != nil {
		return current
	}
	return e.activeCompositeOwner()
}

// moveBoundary is the state a move from current to target stops exiting at: their
// least common ancestor, or the parent of a source that encloses the target, which
// an external transition leaves even so.
func (e *StateExecutor) moveBoundary(current *ast.StateNode, trans *lower.Transition, target *ast.StateNode) *ast.StateNode {
	lca := e.getLCA(current, target)
	if source, isState := trans.Source.(*ast.StateNode); isState && e.encloses(source, target) {
		lca = e.graph.ParentState[source]
	}
	return lca
}

// exitPath lists from and its ancestors up to but excluding stop, innermost first;
// within a region it ends at the region's boundary as well.
func (e *StateExecutor) exitPath(from, stop *ast.StateNode, within *ast.StateRegion) []*ast.StateNode {
	var path []*ast.StateNode
	for current := from; current != nil && current != stop; current = e.graph.ParentState[current] {
		if within != nil && !e.regionContains(within, current) {
			break
		}
		path = append(path, current)
	}
	return path
}

// transitionTo moves the active configuration from the current state along r:
// exit up to the least common ancestor, run the transition effects, then enter
// down to the target, resolving any choice on the way once the effects into it ran.
func (e *StateExecutor) transitionTo(trans *lower.Transition, r route) error {
	return e.transitionToInto(trans, r, nil)
}

// transitionToInto is transitionTo with branches naming the state each
// orthogonal region entered on the way must start in, which is how a history
// pseudostate restores a recorded configuration rather than the initial one.
func (e *StateExecutor) transitionToInto(trans *lower.Transition, r route, branches map[*ast.StateRegion]*ast.StateNode) error {
	currentState := e.moveOrigin()
	return e.travel(trans, currentState, r,
		func(target *ast.StateNode) []*ast.StateNode { return e.exitedByMove(currentState, trans, target) },
		func(target *ast.StateNode) []*ast.StateNode { return e.enteredByMove(currentState, trans, target) },
		func(effects []routeEffect, target *ast.StateNode) error {
			return e.moveTo(trans, currentState, effects, target, branches)
		})
}

// moveTo finishes a move of the single active hierarchy from currentState to
// targetState: the exits still to make, the effects, then the entries.
func (e *StateExecutor) moveTo(trans *lower.Transition, currentState *ast.StateNode, effects []routeEffect, targetState *ast.StateNode, branches map[*ast.StateRegion]*ast.StateNode) error {
	// The trace's source name has to be read before the move, not after it.
	fromName := ""
	if currentState != nil {
		fromName = currentState.Name
	}
	lca := e.moveBoundary(currentState, trans, targetState)

	// Exit states (deepest to shallowest)
	if err := e.exitStates(e.exitPath(currentState, lca, nil)); err != nil {
		return err
	}

	if err := e.runEffects(effects, e.descendantChain(lca, targetState)); err != nil {
		return err
	}
	if lca == targetState {
		return e.completeInto(trans, fromName, targetState)
	}
	return e.enterBelow(trans, fromName, lca, targetState, branches)
}

// enterBelow finishes a move whose exits and effects are done: it enters the
// states below lca down to targetState, then the target's own start.
func (e *StateExecutor) enterBelow(trans *lower.Transition, fromName string, lca, targetState *ast.StateNode, branches map[*ast.StateRegion]*ast.StateNode) error {
	_, leaf, err := e.enterToward(lca, targetState, branches)
	if err != nil {
		return err
	}

	// Each region on the path activates its deepest entered state; a leaf in no
	// region is the machine's single active state.
	onPath := e.branchesTo(nil, leaf)
	for region, state := range onPath {
		e.activeConfig.regionStates[region] = state
	}
	if len(onPath) == 0 && len(e.activeConfig.regionStates) == 0 {
		e.activeConfig.simpleState = leaf
	}
	e.stateStack = e.rootToLeaf(leaf)

	// Schedule new events
	if err := e.scheduleTransitionEvents(); err != nil {
		return fmt.Errorf("schedule events: %w", err)
	}

	if err := e.completeIfDone(leaf); err != nil {
		return fmt.Errorf("complete state machine: %w", err)
	}

	// Record trace
	if e.trace() != nil {
		eventName := triggerName(trans.Trigger)
		e.trace().RecordStateTransition(fromName, targetState.Name, eventName)
	}

	return nil
}

// completeIfDone acts on what entering target completed: a composite state
// whose body reached `done` schedules its own completion transitions, and the
// machine completes once its own body or every top-level region has.
func (e *StateExecutor) completeIfDone(target *ast.StateNode) error {
	if err := e.scheduleCompletedComposites(target); err != nil {
		return err
	}
	if !e.machineComplete() {
		return nil
	}
	// A dispatch pausing at a breakpoint shows the completion vertex it reached;
	// the machine completes once resumed.
	if e.stagedBreakpoint() != nil {
		e.completionDue = true
		return nil
	}
	return e.completeMachine()
}

// completeMachine finishes the machine off its completion vertex: its exit
// behaviors run, it completes and its performance ends.
func (e *StateExecutor) completeMachine() error {
	e.completionDue = false
	if err := e.exitMachine(); err != nil {
		return err
	}
	e.state = StateCompleted
	e.ctx.endPerformanceLife(e.occurrence)
	return nil
}

// terminateMachine ends the machine's performance at the terminate action a
// transition reached (SysML v2 §7.18.3): no state is exited and no exit behavior
// runs; the do behaviors under way are abandoned, and no state stays active.
func (e *StateExecutor) terminateMachine(fromName string, trigger ast.Node, stop *ast.Usage) error {
	name, _ := ast.EffectiveName(stop)
	if e.trace() != nil {
		e.trace().RecordStateTransition(fromName, name, triggerName(trigger))
	}
	abandoned := e.abandonMachine()
	if e.trace() != nil {
		e.trace().RecordStateTerminate(name, abandoned)
	}
	e.state = StateTerminated
	e.ctx.endPerformanceLife(e.occurrence)
	return nil
}

// abandonMachine leaves no state active without exiting any: the do behaviors under
// way end where they are, and their states are returned in entry order.
func (e *StateExecutor) abandonMachine() []string {
	var abandoned []string
	for _, act := range e.doActions {
		abandoned = append(abandoned, act.state.Name)
		if act.run != nil {
			e.endDoRun(act.run)
			act.run = nil
		}
	}
	clear(e.doActions)
	e.doActions = e.doActions[:0]
	e.activeConfig.simpleState = nil
	e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode)
	e.stateStack = nil
	e.completionDue = false
	// Nothing dispatches on an ended machine: what it queued or deferred is discarded.
	e.eventQueue.Withdraw(func(Event) bool { return true })
	e.deferred = e.deferred[:0]
	clear(e.timerScheduled)
	e.changeWaits = nil
	e.machineExited = true
	return abandoned
}

// scheduleCompletedComposites schedules the completion transitions of each
// composite state that entering target completed, in region order.
func (e *StateExecutor) scheduleCompletedComposites(target *ast.StateNode) error {
	var completed []*ast.StateNode
	for _, leaf := range e.activeLeavesBelow(target) {
		composite := e.completedComposite(leaf)
		if composite != nil && !slices.Contains(completed, composite) {
			completed = append(completed, composite)
		}
	}
	for _, composite := range completed {
		if err := e.scheduleCompletionTransitions(composite); err != nil {
			return fmt.Errorf("schedule completion of state %s: %w", composite.Name, err)
		}
	}
	return nil
}

// completedComposite is the declared composite state that leaf, a completion
// vertex just entered, completed: the nearest one enclosing it whose body is
// now complete. It is nil when leaf completes the machine's own body or a
// region of a state still running.
func (e *StateExecutor) completedComposite(leaf *ast.StateNode) *ast.StateNode {
	if !e.graph.Completes(leaf) {
		return nil
	}
	for state := e.graph.ParentState[leaf]; state != nil; state = e.graph.ParentState[state] {
		if !e.stateComplete(state) {
			return nil
		}
		if !e.graph.HiddenStates[state] {
			return state
		}
	}
	return nil
}

// machineComplete reports whether the machine's own body reached `done`, or
// every one of its top-level regions did.
func (e *StateExecutor) machineComplete() bool {
	if len(e.graph.TopRegions) == 0 {
		for _, active := range e.activeStates() {
			if e.graph.Completes(active) && e.graph.ParentState[active] == nil {
				return true
			}
		}
		return false
	}
	for _, region := range e.graph.TopRegions {
		if !e.regionComplete(region) {
			return false
		}
	}
	return true
}

// completeInto finishes a transition into a still active ancestor: the target is
// not re-entered and the region the source left completes (PSSM 8.5.8).
func (e *StateExecutor) completeInto(trans *lower.Transition, fromName string, target *ast.StateNode) error {
	e.stateStack = e.rootToLeaf(target)
	if _, orthogonal := e.graph.CompositeStates[target]; orthogonal {
		if e.stateComplete(target) {
			if err := e.scheduleCompletionTransitions(target); err != nil {
				return fmt.Errorf("schedule completion of state %s: %w", target.Name, err)
			}
		}
	} else {
		// The body completed: its history keeps no substate to restore.
		if record := e.history[target]; record != nil {
			record.child = nil
		}
		onPath := e.branchesTo(nil, target)
		for region, state := range onPath {
			e.activeConfig.regionStates[region] = state
		}
		if len(onPath) == 0 && len(e.activeConfig.regionStates) == 0 {
			e.activeConfig.simpleState = target
		}
		if err := e.scheduleCompletionTransitions(target); err != nil {
			return fmt.Errorf("schedule completion of state %s: %w", target.Name, err)
		}
	}
	if e.trace() != nil {
		e.trace().RecordStateTransition(fromName, target.Name, triggerName(trans.Trigger))
	}
	return nil
}

// regionComplete reports whether region rests at its completion vertex, or at
// its owner once a transition into the owner left it without an active state.
func (e *StateExecutor) regionComplete(region *ast.StateRegion) bool {
	active, ok := e.activeConfig.regionStates[region]
	return ok && (e.graph.Completes(active) || active == e.graph.RegionOwner[region])
}

// stateComplete reports whether state's body has completed: it is a completion
// vertex, its substate rests at its `done`, or its every orthogonal region does.
func (e *StateExecutor) stateComplete(state *ast.StateNode) bool {
	if state == nil {
		return false
	}
	if e.graph.Completes(state) {
		return true
	}
	regions := e.graph.CompositeStates[state]
	if len(regions) == 0 {
		for _, active := range e.activeStates() {
			if e.graph.ParentState[active] == state && e.graph.Completes(active) {
				return true
			}
		}
		return false
	}
	for _, region := range regions {
		if !e.regionComplete(region) {
			return false
		}
	}
	return true
}

// stateCompleted reports whether an active state's completion transitions are
// enabled: its do behavior has finished and its body, where it runs one, is at `done`.
func (e *StateExecutor) stateCompleted(state *ast.StateNode) bool {
	if e.hasRunningDoAction(state) {
		return false
	}
	return !e.bodyRunning(state) || e.stateComplete(state)
}

// bodyRunning reports whether a state nested in state is active.
func (e *StateExecutor) bodyRunning(state *ast.StateNode) bool {
	for _, active := range e.activeStates() {
		if active != state && e.nestedIn(active, state) {
			return true
		}
	}
	return false
}

// isSynchronizationTarget reports whether a transition target is a pseudostate
// that replaces the entire active configuration rather than moving one region:
// fork, join, and history.
func isSynchronizationTarget(target ast.Node) bool {
	ps, ok := target.(*ast.PseudostateNode)
	if !ok {
		return false
	}
	switch ps.Kind {
	case ast.PseudostateFork, ast.PseudostateJoin,
		ast.PseudostateShallowHistory, ast.PseudostateDeepHistory:
		return true
	}
	return false
}

// passesGuard reports whether a transition's guard allows it to fire. A nil
// guard always passes. The guard resolves its names in the scope the transition
// was written in, with the machine's data shadowing it, so a live value wins
// over a same-named declaration.
func (e *StateExecutor) passesGuard(trans *lower.Transition) (bool, error) {
	if trans == nil || trans.Guard == nil {
		return true, nil
	}
	val, err := e.evalStepOf(trans.Source, trans.Guard, trans.BodyScope)
	if err != nil {
		return false, fmt.Errorf("eval guard of %s: %w", transitionDescription(trans), err)
	}
	if val.Kind != ValConst || val.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("guard of %s must be boolean, got %v",
			transitionDescription(trans), val.Kind)
	}
	return val.Const.Bool, nil
}

// transitionDescription names a transition for a diagnostic: by the name it was
// declared with, when it has one, and by the states it runs between otherwise.
func transitionDescription(trans *lower.Transition) string {
	if trans.Name != "" {
		return fmt.Sprintf("transition %s", trans.Name)
	}
	return fmt.Sprintf("transition %s -> %s",
		orAny(StateVertexName(trans.Source)), orAny(StateVertexName(trans.Target)))
}

// recordHistory returns state's history record, creating it on first use.
func (e *StateExecutor) recordHistory(state *ast.StateNode) *historyRecord {
	record, ok := e.history[state]
	if !ok {
		record = &historyRecord{}
		e.history[state] = record
	}
	return record
}

// historyRecorded reports whether state was left in a configuration its history
// restores; a record emptied by completion holds none.
func (e *StateExecutor) historyRecorded(state *ast.StateNode) bool {
	record := e.history[state]
	return record != nil && (record.child != nil || len(record.regions) > 0)
}

// recordChildHistory remembers the substate parent was left in for its history;
// a body left at `done` completed, and a completed configuration leaves none.
func (e *StateExecutor) recordChildHistory(parent, state *ast.StateNode) {
	if e.graph.Completes(state) {
		if record := e.history[parent]; record != nil {
			record.child = nil
		}
		return
	}
	e.recordHistory(parent).child = state
}

// recordRegionHistory remembers the state a region was left in for the owning
// state's history; a region left at `done` completed and is forgotten.
func (e *StateExecutor) recordRegionHistory(region *ast.StateRegion, state *ast.StateNode) {
	owner := e.graph.RegionOwner[region]
	if owner == nil {
		return
	}
	if e.graph.Completes(state) {
		e.forgetRegionHistory(region)
		return
	}
	record := e.recordHistory(owner)
	if record.regions == nil {
		record.regions = make(map[*ast.StateRegion]*ast.StateNode)
	}
	record.regions[region] = state
}

// forgetRegionHistory drops a region's recorded state, for a region left with no
// active state at all: there is nothing for a history pseudostate to restore.
func (e *StateExecutor) forgetRegionHistory(region *ast.StateRegion) {
	owner := e.graph.RegionOwner[region]
	if owner == nil {
		return
	}
	if record := e.history[owner]; record != nil {
		delete(record.regions, region)
	}
}

// fireHistoryTransition takes a transition into a history pseudostate: the
// composite state that owns it is re-entered in the configuration it was last
// left in. Before the state has ever been exited there is nothing to restore, so
// the history's own outgoing transition supplies the default target, as UML's
// default history transition does (UML is the reference: no SysML v2 notation);
// without one the owner is entered as any transition into it would enter it,
// through its entry transitions.
//
// A shallow history restores the substate that was active; a deep history keeps
// descending, restoring the innermost one.
func (e *StateExecutor) fireHistoryTransition(trans *lower.Transition, hist *ast.PseudostateNode, r route) error {
	owner, err := e.historyOwner(hist)
	if err != nil {
		return err
	}
	currentState := e.moveOrigin()
	return e.travelChoosing(e.drawsBeyond(hist) || e.mayDrawEntering(owner), trans, currentState, r,
		func(*ast.StateNode) []*ast.StateNode { return e.exitedByMove(currentState, trans, owner) },
		func(*ast.StateNode) []*ast.StateNode { return e.enteredByMove(currentState, trans, owner) },
		func(effects []routeEffect, _ *ast.StateNode) error {
			return e.moveToHistory(trans, currentState, effects, hist, owner)
		})
}

// historyOwner is the composite state hist restores; a history in the machine's
// own body restores the top-level configuration, kept under the root state.
func (e *StateExecutor) historyOwner(hist *ast.PseudostateNode) (*ast.StateNode, error) {
	if owner := e.graph.PseudostateOwner[hist]; owner != nil {
		return owner, nil
	}
	if e.graph.Machine != nil && len(e.graph.TopRegions) == 0 {
		return e.graph.Machine, nil
	}
	return nil, fmt.Errorf("history %s must be declared inside the composite state it restores", hist.Name)
}

// historyBoundary is the state a move into owner's history stops exiting at and
// enters from: the owner's parent, or the root for the machine's own body.
func (e *StateExecutor) historyBoundary(currentState *ast.StateNode, trans *lower.Transition, owner *ast.StateNode) *ast.StateNode {
	if owner == e.graph.Machine {
		return nil
	}
	return e.moveBoundary(currentState, trans, owner)
}

// moveToHistory finishes a move into hist: exits run first, since leaving the
// owner writes the record, then the record is read and the owner re-entered. An
// effect enclosed by a state on the way down to the owner runs as it is entered.
func (e *StateExecutor) moveToHistory(trans *lower.Transition, currentState *ast.StateNode, effects []routeEffect, hist *ast.PseudostateNode, owner *ast.StateNode) error {
	fromName := ""
	if currentState != nil {
		fromName = currentState.Name
	}
	lca := e.historyBoundary(currentState, trans, owner)
	leaving := e.exitPath(currentState, lca, nil)
	// A record the exits leave as it is is checked before anything moves, so an
	// unenterable history fails with the machine where it was.
	if owner != e.graph.Machine && !slices.Contains(leaving, owner) {
		if _, _, err := e.historyEntry(hist, owner); err != nil {
			return err
		}
	}
	if err := e.exitStates(leaving); err != nil {
		return err
	}
	if err := e.runEffects(effects, e.descendantChain(lca, owner)); err != nil {
		return err
	}

	target, branches, err := e.historyEntry(hist, owner)
	if err != nil {
		return err
	}
	if target != nil {
		return e.enterBelow(trans, fromName, lca, target, branches)
	}
	below := lca
	if owner != e.graph.Machine {
		for _, state := range e.descendantChain(lca, owner) {
			if err := e.enterStateInto(state, nil); err != nil {
				return fmt.Errorf("enter state: %w", err)
			}
		}
		below = owner
	}
	r, err := e.defaultHistoryRoute(hist, below)
	if err != nil {
		return err
	}
	if r.terminate != nil {
		return e.terminateAt(trans, fromName, r, e.descendantChain(below, e.graph.TerminateOwner[r.terminate]))
	}
	if err := e.runEffects(r.effects(e.graph), e.descendantChain(below, r.target)); err != nil {
		return err
	}
	e.noteFired(r.segments...)
	return e.enterBelow(trans, fromName, below, r.target, nil)
}

// defaultHistoryRoute takes a history's default transition from inside its
// owner, drawing at once among several branches enabled, as what it notes is
// noted, and resolving any choice on the way once the effects into it have run;
// each stretch of segments is recorded as fired once its effects are done. The
// effects of the settled rest are the caller's to run and record as it enters
// below owner.
func (e *StateExecutor) defaultHistoryRoute(hist *ast.PseudostateNode, owner *ast.StateNode) (route, error) {
	r, err := e.followOut(hist, route{})
	if err == nil {
		r, err = e.settleDraws(r)
	}
	e.ctx.noteAll(r.notes)
	r.notes = nil
	if err != nil {
		return route{}, fmt.Errorf("default transition of history %s: %w", hist.Name, err)
	}
	for r.choice != nil {
		targets, stops, err := e.reachable(r)
		if err != nil {
			return route{}, err
		}
		var ends routeEnds
		for _, target := range targets {
			ends.entries = append(ends.entries, e.descendantChain(owner, target))
		}
		for _, stop := range stops {
			ends.entries = append(ends.entries, e.descendantChain(owner, e.graph.TerminateOwner[stop]))
		}
		certain := ends.certainEntries()
		if err := e.runEffects(r.effects(e.graph), certain); err != nil {
			return route{}, err
		}
		if err := e.enterOwnerOf(r.choice, certain); err != nil {
			return route{}, err
		}
		e.noteFired(r.segments...)
		if r, err = e.resolveChoice(r); err != nil {
			return route{}, err
		}
	}
	return r, nil
}

// drawsBeyond reports whether a path out of ps through junctions may face a draw
// the policy makes while the move is under way: a choice, or several branches out
// of ps or a junction beyond it.
func (e *StateExecutor) drawsBeyond(ps *ast.PseudostateNode) bool {
	seen := map[*ast.PseudostateNode]bool{ps: true}
	var visit func(ps *ast.PseudostateNode) bool
	visit = func(ps *ast.PseudostateNode) bool {
		branches := e.graph.Transitions[ps]
		if len(branches) > 1 {
			return true
		}
		for _, branch := range branches {
			next, ok := branch.Target.(*ast.PseudostateNode)
			if !ok || seen[next] {
				continue
			}
			seen[next] = true
			if next.Kind == ast.PseudostateChoice || visit(next) {
				return true
			}
		}
		return false
	}
	return visit(ps)
}

// historyEntry is the state a move into owner's history enters and each region's
// branch, read after the exits; a nil state says to take the default transition.
func (e *StateExecutor) historyEntry(hist *ast.PseudostateNode, owner *ast.StateNode) (*ast.StateNode, map[*ast.StateRegion]*ast.StateNode, error) {
	if !e.historyRecorded(owner) {
		if len(e.graph.Transitions[hist]) > 0 {
			return nil, nil, nil
		}
		if owner == e.graph.Machine || !e.hasDefaultEntry(owner) {
			return nil, nil, fmt.Errorf("%w: history %s has no default transition, %s has no recorded configuration and declares no entry transition",
				ErrHistoryWithoutEntry, hist.Name, owner.Name)
		}
		return owner, nil, nil
	}

	record := e.history[owner]
	deep := hist.Kind == ast.PseudostateDeepHistory
	branches := make(map[*ast.StateRegion]*ast.StateNode)

	if len(record.regions) > 0 {
		// The owner keeps its configuration in its regions, so it is re-entered
		// with one branch per region rather than moved to a single state.
		for _, region := range e.graph.CompositeStates[owner] {
			active, recorded := record.regions[region]
			if !recorded {
				continue
			}
			if deep {
				active = e.deepestRecorded(active, branches)
			}
			branches[region] = active
		}
		return owner, branches, nil
	}

	target := record.child
	if deep {
		target = e.deepestRecorded(target, branches)
	}
	return target, branches, nil
}

// hasDefaultEntry reports whether entering state with no branch chosen has a
// state to start in: an entry transition of its body, or of each of its regions.
func (e *StateExecutor) hasDefaultEntry(state *ast.StateNode) bool {
	regions, orthogonal := e.graph.CompositeStates[state]
	if !orthogonal {
		return len(e.graph.StartOf(state)) > 0
	}
	for _, region := range regions {
		if e.graph.RegionState[region] == nil && len(e.graph.StartOf(region)) == 0 {
			return false
		}
	}
	return true
}

// deepestRecorded follows the configuration recorded below state and returns the
// innermost state to enter, adding a branch for every orthogonal region it
// passes through so those regions are restored too. A state whose recorded
// configuration lives in regions is itself the state to enter, since its regions
// carry the rest.
func (e *StateExecutor) deepestRecorded(state *ast.StateNode, branches map[*ast.StateRegion]*ast.StateNode) *ast.StateNode {
	for {
		record := e.history[state]
		if record == nil {
			return state
		}
		if len(record.regions) > 0 {
			for _, region := range e.graph.CompositeStates[state] {
				if active, recorded := record.regions[region]; recorded {
					branches[region] = e.deepestRecorded(active, branches)
				}
			}
			return state
		}
		if record.child == nil {
			return state
		}
		state = record.child
	}
}

// fireForkTransition takes a transition into a fork: every outgoing branch is
// taken at once, making one state active per orthogonal region of the composite
// state that owns them. The move is one whole: a refused draw among the branches
// undoes the exits and effects ahead of it.
func (e *StateExecutor) fireForkTransition(trans *lower.Transition, fork *ast.PseudostateNode) error {
	plan, err := e.forkPlan(fork)
	if err != nil {
		return err
	}
	owner := plan.Owner
	e.noteFired(trans)
	e.noteFired(e.graph.Transitions[fork]...)
	return e.moveWhole(func() error { return e.forkMove(trans, fork, plan, owner) })
}

// forkMove is the fork's compound transition: the exits, the effect, the branches.
func (e *StateExecutor) forkMove(trans *lower.Transition, fork *ast.PseudostateNode, plan *lower.ForkPlan, owner *ast.StateNode) error {
	// Leave the source configuration down to the move's boundary, which stays
	// active: states above it are neither exited nor entered again.
	boundary, err := e.leaveForFork(trans, owner)
	if err != nil {
		return err
	}
	for _, behavior := range trans.Effect {
		if err := e.executeBehavior(behavior); err != nil {
			return fmt.Errorf("transition effect: %w", err)
		}
	}

	// The fork's own parent is entered before its branches fire; the branches
	// enter the rest of the way down to the owner and their targets, bypassing
	// the initial states of the regions they enter (PSSM §8.5.7).
	above := e.forkEntry(boundary, owner)
	if err := e.enterAboveFork(fork, above); err != nil {
		return err
	}
	if err := e.enterForkBranches(fork, plan, above); err != nil {
		return err
	}
	// A region the move re-entered on its way down keeps the state of that path.
	for region, state := range e.branchesTo(boundary, owner) {
		if _, active := e.activeConfig.regionStates[region]; !active {
			e.activeConfig.regionStates[region] = state
		}
	}

	if err := e.scheduleTransitionEvents(); err != nil {
		return fmt.Errorf("schedule events: %w", err)
	}
	// Branches ending in `done` complete the owner, or the machine, at once.
	if err := e.completeIfDone(owner); err != nil {
		return fmt.Errorf("complete state machine: %w", err)
	}
	if e.trace() != nil {
		e.trace().RecordStateTransition(StateVertexName(trans.Source), fork.Name, "")
	}
	return nil
}

// leaveForFork exits the source configuration of a transition into a fork whose
// branches enter owner's regions, and returns the state the exits stopped at: the
// least common ancestor of source and owner, as for a move to a single state.
func (e *StateExecutor) leaveForFork(trans *lower.Transition, owner *ast.StateNode) (*ast.StateNode, error) {
	source, _ := trans.Source.(*ast.StateNode)
	region := e.activeRegionOf(source)
	if region == nil {
		origin := e.moveOrigin()
		lca := e.moveBoundary(origin, trans, owner)
		return lca, e.exitStates(e.exitPath(origin, lca, nil))
	}
	sourceRegion, targetRegion := e.regionMove(region, owner)
	if targetRegion == nil {
		regionOwner := e.graph.RegionOwner[region]
		if regionOwner == nil {
			return nil, fmt.Errorf("fork into %s from region %s: the target lies outside the machine's regions", owner.Name, region.Name)
		}
		lca := e.getLCA(regionOwner, owner)
		if lca == owner {
			return owner, e.exitRegionsOf(owner)
		}
		return lca, e.exitRegionOwnerTo(regionOwner, lca)
	}
	keep := e.regionKeep(targetRegion, trans, owner)
	if sourceRegion != targetRegion {
		if err := e.exitRegionTo(sourceRegion, nil); err != nil {
			return nil, err
		}
	}
	return keep, e.exitRegionTo(targetRegion, keep)
}

// exitRegionsOf leaves every region of owner, which stays active, in declaration
// order: a fork reached from inside them restarts them all from its branches.
func (e *StateExecutor) exitRegionsOf(owner *ast.StateNode) error {
	for _, region := range e.graph.CompositeStates[owner] {
		if err := e.exitRegionTo(region, owner); err != nil {
			return err
		}
	}
	return nil
}

// regionsExitPath lists the states exitRegionsOf exits.
func (e *StateExecutor) regionsExitPath(owner *ast.StateNode) []*ast.StateNode {
	var exited []*ast.StateNode
	for _, region := range e.graph.CompositeStates[owner] {
		exited = append(exited, e.regionExitPath(region, owner)...)
	}
	return exited
}

// forkPlan is where a fork's branches lead, as lowering checked and recorded it.
func (e *StateExecutor) forkPlan(fork *ast.PseudostateNode) (*lower.ForkPlan, error) {
	plan := e.graph.ForkPlans[fork]
	if plan == nil {
		return nil, fmt.Errorf("fork %s has no lowered plan", fork.Name)
	}
	return plan, nil
}

// fireJoinTransition takes a transition into a join, reporting whether the join
// fired. It fires only while the occurrence firing trans enables every other
// segment into the join, still, as it fires; until then the segment simply
// waits, and what selecting it noted is recorded only once it fires.
func (e *StateExecutor) fireJoinTransition(trans *lower.Transition, join *ast.PseudostateNode, r route) (bool, error) {
	if !r.settled() {
		return false, nil
	}
	if ready, err := e.joinSynchronized(trans, e.firingEvent); err != nil || !ready {
		return false, err
	}
	plan, err := e.joinPlan(join)
	if err != nil {
		return false, err
	}

	// Every incoming segment fires, then the move goes on from the state whose
	// regions they left so the usual hierarchy walk exits it as well.
	r.segments = r.segments[1:]
	return true, e.moveWhole(func() error {
		// Decided inside the move: a refused draw undoes the selection's records too.
		r = e.transitionDecided(r)
		if err := e.fireJoinIncoming(join, plan); err != nil {
			return err
		}
		return e.leaveJoinOwner(plan.Owner, trans, r)
	})
}

// joinPlan is where a join's segments come from, as lowering checked and recorded it.
func (e *StateExecutor) joinPlan(join *ast.PseudostateNode) (*lower.JoinPlan, error) {
	plan := e.graph.JoinPlans[join]
	if plan == nil {
		return nil, fmt.Errorf("join %s has no lowered plan", join.Name)
	}
	return plan, nil
}

// leaveJoinOwner finishes a join's compound transition once its segments have
// fired: the move goes on out of owner, whose regions they left, along r.
func (e *StateExecutor) leaveJoinOwner(owner *ast.StateNode, trans *lower.Transition, r route) error {
	if owner == nil {
		// The machine's own regions were joined: the rest of them are left too.
		for _, region := range e.graph.TopRegions {
			if err := e.exitRegionTo(region, nil); err != nil {
				return err
			}
		}
		e.activeConfig.simpleState = nil
		return e.transitionTo(trans, r)
	}
	if region := e.activeRegionOf(owner); region != nil {
		return e.travel(trans, owner, r,
			func(target *ast.StateNode) []*ast.StateNode { return e.exitedInRegion(region, trans, target) },
			func(target *ast.StateNode) []*ast.StateNode { return e.enteredInRegion(region, trans, target) },
			func(effects []routeEffect, target *ast.StateNode) error {
				return e.moveInRegion(region, owner, trans, effects, target)
			})
	}
	e.activeConfig.simpleState = owner
	return e.transitionTo(trans, r)
}

// joinExits lists the states firing join exits beyond its sources: the states
// between each source and the owner, then those the move out of the owner exits.
func (e *StateExecutor) joinExits(plan *lower.JoinPlan, trans *lower.Transition, r route) ([]*ast.StateNode, bool) {
	var exited []*ast.StateNode
	for _, segment := range e.joinIncoming(trans.Target.(*ast.PseudostateNode)) {
		exited = append(exited, e.exitPath(e.joinSegmentLeaves(segment, plan), plan.Owner, nil)...)
	}
	if plan.Owner == nil {
		for _, region := range e.graph.TopRegions {
			exited = append(exited, e.regionExitPath(region, nil)...)
		}
		beyond, ok := e.mayExit(r, trans, nil, func(target *ast.StateNode) []*ast.StateNode { return e.exitedByMove(nil, trans, target) })
		return append(exited, beyond...), ok
	}
	if region := e.activeRegionOf(plan.Owner); region != nil {
		beyond, ok := e.mayExit(r, trans, plan.Owner, func(target *ast.StateNode) []*ast.StateNode { return e.exitedInRegion(region, trans, target) })
		return append(exited, beyond...), ok
	}
	beyond, ok := e.mayExit(r, trans, plan.Owner, func(target *ast.StateNode) []*ast.StateNode { return e.exitedByMove(plan.Owner, trans, target) })
	return append(exited, beyond...), ok
}

// fireJoinIncoming fires each transition into join whole — its source exited,
// then its effect — in an order the policy draws among their sources.
func (e *StateExecutor) fireJoinIncoming(join *ast.PseudostateNode, plan *lower.JoinPlan) error {
	pending := e.joinIncoming(join)
	for len(pending) > 0 {
		next := 0
		if len(pending) > 1 {
			sources := make([]*ast.StateNode, len(pending))
			for i, trans := range pending {
				sources[i] = trans.Source.(*ast.StateNode)
			}
			choice := e.regionOrderChoice("join "+join.Name, sources)
			if err := e.ctx.scheduling().refusal(); err != nil {
				return err
			}
			e.ctx.noteChoice(choice)
			next = choice.Taken
		}
		trans := pending[next]
		pending = slices.Delete(pending, next, next+1)
		if err := e.fireJoinSegment(trans, plan); err != nil {
			return err
		}
	}
	return nil
}

// fireJoinSegment fires one transition into a join: its source and the states
// between it and the owner exited, innermost first, then its effect, with the
// arguments its own trigger takes from the occurrence bound; it is recorded as taken.
func (e *StateExecutor) fireJoinSegment(trans *lower.Transition, plan *lower.JoinPlan) error {
	source := trans.Source.(*ast.StateNode)
	if e.firingEvent != nil && trans.Trigger != nil {
		unbind, err := e.bindTriggerArguments(trans, e.firingEvent)
		if err != nil {
			unbind()
			return fmt.Errorf("state %s: %w", source.Name, err)
		}
		defer unbind()
	}
	leaving := e.joinSegmentLeaves(trans, plan)
	// The region is left whole: its configuration is what a history of the owner
	// restores, and the entry goes before the exits or the owner's exit walks it again.
	if region := plan.Regions[trans]; region != nil {
		if active, isActive := e.activeConfig.regionStates[region]; isActive {
			e.recordRegionHistory(region, active)
			delete(e.activeConfig.regionStates, region)
		}
	}
	if err := e.exitStates(e.exitPath(leaving, plan.Owner, nil)); err != nil {
		return err
	}
	if err := e.runBehaviors(trans.Effect); err != nil {
		return err
	}
	e.noteFired(trans)
	return nil
}

// joinSegmentLeaves is the state a join segment's exit starts from: its region's
// active state when that lies below the segment's composite source, else the source.
func (e *StateExecutor) joinSegmentLeaves(segment *lower.Transition, plan *lower.JoinPlan) *ast.StateNode {
	source := segment.Source.(*ast.StateNode)
	if active, ok := e.activeConfig.regionStates[plan.Regions[segment]]; ok && e.isBelowOrEqual(active, source) {
		return active
	}
	return source
}

// joinIncoming lists the transitions into join, in source declaration order;
// joinSources has checked that each source is a state.
func (e *StateExecutor) joinIncoming(join *ast.PseudostateNode) []*lower.Transition {
	var incoming []*lower.Transition
	for _, state := range e.graph.States {
		for _, trans := range e.graph.Transitions[state] {
			if trans.Target == ast.Node(join) {
				incoming = append(incoming, trans)
			}
		}
	}
	return incoming
}

// joinSynchronized reports whether a transition is enabled as far as its target
// goes: one into a join only while every other segment into the join is enabled
// too — its source active, its guard holding, and its trigger, if it has one,
// taking the occurrence, or its source completed if it has none. Every path
// firing a join, dispatched on a signal, call, timer, completion or change,
// goes through this.
func (e *StateExecutor) joinSynchronized(trans *lower.Transition, event *Event) (bool, error) {
	join, ok := trans.Target.(*ast.PseudostateNode)
	if !ok || join.Kind != ast.PseudostateJoin {
		return true, nil
	}
	if _, err := e.joinSources(join); err != nil {
		return false, err
	}
	for _, segment := range e.joinIncoming(join) {
		if segment == trans {
			continue
		}
		if !e.inActiveConfiguration(segment.Source.(*ast.StateNode)) {
			return false, nil
		}
		if segment.Trigger == nil {
			if !e.stateCompleted(segment.Source.(*ast.StateNode)) {
				return false, nil
			}
		} else {
			takes, err := e.segmentTakes(segment, event)
			if err != nil || !takes {
				return false, err
			}
		}
		pass, err := e.segmentGuardHolds(segment, event)
		if err != nil || !pass {
			return false, err
		}
	}
	return true, nil
}

// segmentTakes reports whether a join segment's trigger takes the dispatched
// occurrence. Each timer is its own occurrence, so a time-triggered segment takes
// another timer's expiry while its own timer is due: the expiries at one instant
// are one occurrence for the join, which a signal, call or completion dispatched
// then is not.
func (e *StateExecutor) segmentTakes(segment *lower.Transition, event *Event) (bool, error) {
	if event == nil {
		return false, nil
	}
	if _, isTime := segment.Trigger.(*ast.TimeEvent); isTime {
		if !isTimerExpiry(*event) {
			return false, nil
		}
		timer, running := e.eventQueue.TimerOf(segment)
		return running && timer.Timestamp <= event.Timestamp, nil
	}
	return e.matchesEvent(segment, event)
}

// segmentGuardHolds reads a join segment's guard with its trigger's arguments bound
// and unbound again, as transitionEnabled reads the selected transition's.
func (e *StateExecutor) segmentGuardHolds(segment *lower.Transition, event *Event) (bool, error) {
	if segment.Trigger == nil || event == nil {
		return e.passesGuard(segment)
	}
	unbind, err := e.bindTriggerArguments(segment, event)
	defer unbind()
	if err != nil {
		return false, err
	}
	return e.passesGuard(segment)
}

// allActive reports whether every state is part of the active configuration,
// itself active or enclosing an active state.
func (e *StateExecutor) allActive(states []*ast.StateNode) bool {
	for _, state := range states {
		if !e.inActiveConfiguration(state) {
			return false
		}
	}
	return true
}

// joinSources returns the source state of every transition into join, in state
// declaration order. That order is observable — it is the order the branches are
// exited in — so it comes from graph.States rather than from the graph.Transitions
// map, whose iteration order varies between runs.
func (e *StateExecutor) joinSources(join *ast.PseudostateNode) ([]*ast.StateNode, error) {
	incoming := e.joinIncoming(join)
	sources := make([]*ast.StateNode, len(incoming))
	for i, trans := range incoming {
		sources[i] = trans.Source.(*ast.StateNode)
	}
	for _, ps := range e.graph.Pseudostates {
		for _, trans := range e.graph.Transitions[ps] {
			if trans.Target == ast.Node(join) {
				return nil, fmt.Errorf("join %s: incoming source must be a state, got %T", join.Name, trans.Source)
			}
		}
	}
	if len(sources) < 2 {
		return nil, fmt.Errorf("join %s needs at least two incoming transitions, found %d", join.Name, len(sources))
	}
	return sources, nil
}

// isActive reports whether state is part of the active configuration.
func (e *StateExecutor) isActive(state *ast.StateNode) bool {
	if e.activeConfig.simpleState == state {
		return true
	}
	for _, active := range e.activeConfig.regionStates {
		if active == state {
			return true
		}
	}
	return false
}

// activeCompositeOwner returns the deepest composite state whose orthogonal
// regions hold the active configuration, or nil when no region is active.
func (e *StateExecutor) activeCompositeOwner() *ast.StateNode {
	var deepest *ast.StateNode
	depth := -1
	for _, state := range e.graph.CompositeStateOrder {
		regions := e.graph.CompositeStates[state]
		for _, region := range regions {
			if _, active := e.activeConfig.regionStates[region]; active {
				if d := len(e.getParentChain(state)); d > depth {
					deepest, depth = state, d
				}
				break
			}
		}
	}
	return deepest
}

// orderedRegionStates returns the active state of each orthogonal region in
// region declaration order, since exit behaviors run in the order returned.
func (e *StateExecutor) orderedRegionStates() []*ast.StateNode {
	regions := e.orderedActiveRegions()
	states := make([]*ast.StateNode, 0, len(regions))
	for _, region := range regions {
		states = append(states, e.activeConfig.regionStates[region])
	}
	return states
}

// inActiveConfiguration reports whether state is active, either as an active
// state itself or as an ancestor of one.
func (e *StateExecutor) inActiveConfiguration(state *ast.StateNode) bool {
	for _, active := range e.activeStates() {
		for _, ancestor := range e.getParentChain(active) {
			if ancestor == state {
				return true
			}
		}
	}
	return false
}

// RunToCompletion processes queued events until the machine completes or has no
// event or running do behavior left, at which point it suspends. A state's do
// behavior runs while the state is active: each run-to-completion step advances
// every active state's do behavior by one action and then dispatches one event,
// so concurrently active states interleave instead of one running to the end at
// entry, and leaving a state abandons the rest of its do behavior.
//
// Change conditions are re-tested per micro-step — after the do round, before
// the next queued event, and again at quiescence — a tool-defined cadence, since
// KerML has no clock (docs/project/spec-compliance.md).
//
// The run is bounded by the context's event and do action budgets
// (OPENSYSML_MAX_EVENTS, OPENSYSML_MAX_DO_STEPS), so a cyclic machine reports a typed
// error instead of spinning forever. A poll that fires nothing costs no budget;
// a change transition taken counts as one step, like a dispatched event.
func (e *StateExecutor) RunToCompletion() error {
	return e.run(false)
}

// RunToQuiescence runs the machine as RunToCompletion does, but leaves a timer
// set for later waiting on the clock rather than advancing to it: the
// configuration an object settles into is the one reached at the time it was
// materialized, and a timer it is waiting on is driven by advancing the clock.
func (e *StateExecutor) RunToQuiescence() error {
	return e.run(true)
}

// run is the run-to-completion loop; atCurrentTime holds the clock, otherwise
// once nothing is due it advances to the earliest wait, running whatever is due there.
func (e *StateExecutor) run(atCurrentTime bool) error {
	var progress dueProgress
	return e.runCounting(atCurrentTime, &progress)
}

func (e *StateExecutor) runCounting(atCurrentTime bool, progress *dueProgress) (err error) {
	defer e.ctx.beginExecutorRun(&e.driven)()
	defer e.completedWhole(&err)
	wasRunning := e.inRun
	e.inRun = true
	defer func() { e.inRun = wasRunning }()

	// Suspension is derived at quiescence, so re-running is allowed: a run that
	// finds nothing to do suspends again.
	if e.state == StateSuspended {
		e.state, e.pausedAt = StateRunning, nil
	}

	for e.state == StateRunning {
		stepped, err := e.runStep(progress)
		if err != nil {
			return err
		}
		if stepped {
			progress.unsettle()
			continue
		}
		if atCurrentTime {
			e.state = StateSuspended
			return nil
		}
		// Nothing left at this instant: the others due run, then the clock moves to the
		// earliest wait. Only a machine with a timer of its own running moves the clock.
		progress.settle(e)
		for {
			picked, err := e.ctx.runDue(e, progress)
			if err != nil {
				return err
			}
			// Its turn: work is due, or a change condition it watches is to be polled.
			if picked || e.dueWork() {
				break
			}
			if _, waiting := e.NextWait(); !waiting || !e.ctx.advanceToNextDue(progress) {
				e.state = StateSuspended
				return nil
			}
		}
	}
	return nil
}

// dueLabel names the machine in a due-order choice.
func (e *StateExecutor) dueLabel() string {
	return "state machine " + symbolText(e.stateMachine) + performerSuffix(e.self)
}

// clockWaits lists the timers set for an instant the clock has not reached, and
// the waits the do behaviors under way are paused on.
func (e *StateExecutor) clockWaits() []ClockWait {
	return notYetDue(e.armedWaits(), e.ctx.clock.now)
}

// armedWaits lists the timers set and the do behaviors' waits on the clock, due
// or not, earliest first.
func (e *StateExecutor) armedWaits() []ClockWait {
	return e.armed((*doRun).armedWaits)
}

// visibleArmedWaits lists armedWaits and, through the do behaviors' paused work,
// the waits of the actions it performs.
func (e *StateExecutor) visibleArmedWaits() []ClockWait {
	return e.armed((*doRun).visibleArmedWaits)
}

// armed lists the timers set and, of each do behavior under way, the waits ofRun lists.
func (e *StateExecutor) armed(ofRun func(*doRun) []ClockWait) []ClockWait {
	var waits []ClockWait
	for _, event := range e.eventQueue.events {
		trans, ok := event.Payload.(*lower.Transition)
		if !ok {
			continue
		}
		waits = append(waits, ClockWait{Due: event.Timestamp, holder: e, what: transitionWait{trans}})
	}
	for _, act := range e.doActions {
		if act.run == nil {
			continue
		}
		for _, wait := range ofRun(act.run) {
			waits = append(waits, ClockWait{Due: wait.Due, holder: e, what: wait.what})
		}
	}
	slices.SortStableFunc(waits, func(a, b ClockWait) int { return cmp.Compare(a.Due, b.Due) })
	return waits
}

// transitionWait describes a timed transition armed on the clock.
type transitionWait struct {
	trans *lower.Transition
}

func (w transitionWait) String() string {
	return fmt.Sprintf("%s -> %s", triggerName(w.trans.Trigger), StateVertexName(w.trans.Target))
}

// dueWork reports an event due, a signal in flight this machine takes, or a do
// action left to run, on a machine the clock may drive.
func (e *StateExecutor) dueWork() bool {
	if !e.drivable() {
		return false
	}
	return e.completionDue || e.hasDueEvent() || e.hasPendingSignal() || e.HasPendingDoWork()
}

// watchesChange reports a change condition the active configuration waits on.
func (e *StateExecutor) watchesChange() bool {
	return e.drivable() && e.WatchesChangeCondition()
}

// drivable is a machine the clock may run: initialized, not completed, and not
// paused at a breakpoint, which holds its work until its driver resumes it.
func (e *StateExecutor) drivable() bool {
	return (e.state == StateRunning || e.state == StateSuspended) && e.pausedAt == nil
}

// runDue runs the machine to quiescence at the current instant.
func (e *StateExecutor) runDue(progress *dueProgress) (bool, error) {
	before := *progress
	err := e.runCounting(true, progress)
	return progress.events > before.events || progress.doSteps > before.doSteps, err
}

// runOne runs one atomic unit of the machine's work at the current instant, in
// runStep's order: one do action of the round under way, then — the round closed —
// a risen change condition or the next due event, else the next round's first action.
func (e *StateExecutor) runOne(progress *dueProgress) (moved bool, err error) {
	defer e.ctx.beginExecutorRun(&e.driven)()
	defer e.completedWhole(&err)
	wasRunning := e.inRun
	e.inRun = true
	defer func() { e.inRun = wasRunning }()
	if e.state == StateSuspended {
		e.state, e.pausedAt = StateRunning, nil
	}
	if e.state != StateRunning {
		return false, nil
	}
	if e.completionDue {
		return true, e.completeMachine()
	}
	if !e.roundDone {
		if stepped, err := e.stepRound(progress); err != nil || stepped {
			return stepped, err
		}
	}
	e.roundDone = false
	dispatched, err := e.dispatchOne(progress)
	if err != nil || dispatched {
		return dispatched, err
	}
	stepped, err := e.stepRound(progress)
	if err != nil {
		return false, err
	}
	if !stepped {
		e.roundDone = false
		e.state = StateSuspended
	}
	return stepped, nil
}

// stepRound runs one do action of the round under way, opening a round of the do
// actions due when none is; false, the round closed, when nothing is left to act.
func (e *StateExecutor) stepRound(progress *dueProgress) (bool, error) {
	if len(e.round) == 0 {
		for _, act := range e.doActions {
			if act.due(e.ctx) {
				e.round = append(e.round, act)
			}
		}
	}
	e.round = slices.DeleteFunc(e.round, func(act *doAction) bool { return !e.isRunningDoAction(act) })
	if len(e.round) == 0 {
		e.roundDone = true
		return false, nil
	}
	next, err := e.chooseDoAction(e.round)
	if err != nil {
		return false, err
	}
	act := e.round[next]
	e.round = slices.Delete(e.round, next, next+1)
	if err := e.stepDoAction(act, func(run *doRun) (*doRun, error) { return run.resume(e.ctx) }); err != nil {
		return false, err
	}
	progress.doSteps++
	if progress.doSteps >= e.ctx.maxDoSteps {
		return false, budgetExceeded(ErrDoStepLimitExceeded,
			fmt.Sprintf("state machine exceeded max do action steps (%d steps; raise %s to allow more), possible non-terminating do behavior",
				e.ctx.maxDoSteps, MaxDoStepsEnvVar))
	}
	if len(e.round) == 0 {
		e.roundDone = true
		return true, e.settleDoActions()
	}
	return true, nil
}

// dispatchOne is runStep's dispatch phase: a risen change condition fires, else
// the next due event is dispatched; false when neither is there.
func (e *StateExecutor) dispatchOne(progress *dueProgress) (bool, error) {
	maxStateEvents := e.ctx.maxStateEvents
	fired, err := e.pollChangeEvents()
	if err != nil {
		return false, fmt.Errorf("poll change conditions: %w", err)
	}
	if fired {
		if progress.events >= maxStateEvents {
			return false, e.eventBudgetExceeded(maxStateEvents)
		}
		progress.events++
		return true, nil
	}
	delivered, err := e.deliverPendingSignal()
	if err != nil || !delivered {
		return false, err
	}
	if progress.events >= maxStateEvents {
		return false, e.eventBudgetExceeded(maxStateEvents)
	}
	progress.events++
	if err := e.processNextEvent(); err != nil {
		return false, fmt.Errorf("process event: %w", err)
	}
	progress.noteDispatch(*e.lastDispatch)
	return true, nil
}

func (e *StateExecutor) finished() bool { return e.state.Ended() }
func (e *StateExecutor) running() bool  { return e.inRun }

// runStep is one run-to-completion step (a do round, a risen change condition,
// else the next due event); false when nothing was left to do at this instant.
func (e *StateExecutor) runStep(progress *dueProgress) (bool, error) {
	if e.completionDue {
		return true, e.completeMachine()
	}
	maxStateEvents, maxDoSteps := e.ctx.maxStateEvents, e.ctx.maxDoSteps
	ran, err := e.runDoRound()
	if err != nil {
		return false, err
	}
	progress.doSteps += int64(ran)
	if progress.doSteps >= maxDoSteps {
		return false, budgetExceeded(ErrDoStepLimitExceeded,
			fmt.Sprintf("state machine exceeded max do action steps (%d steps; raise %s to allow more), possible non-terminating do behavior",
				maxDoSteps, MaxDoStepsEnvVar))
	}
	fired, err := e.pollChangeEvents()
	if err != nil {
		return false, fmt.Errorf("poll change conditions: %w", err)
	}
	if fired {
		if progress.events >= maxStateEvents {
			return false, e.eventBudgetExceeded(maxStateEvents)
		}
		progress.events++
		return true, nil
	}
	delivered, err := e.deliverPendingSignal()
	if err != nil {
		return false, err
	}
	if !delivered {
		return ran > 0, nil // do behaviors still running may yet queue events
	}
	if progress.events >= maxStateEvents {
		return false, e.eventBudgetExceeded(maxStateEvents)
	}
	progress.events++
	if err := e.processNextEvent(); err != nil {
		return false, fmt.Errorf("process event: %w", err)
	}
	progress.noteDispatch(*e.lastDispatch)
	return true, nil
}

func (e *StateExecutor) eventBudgetExceeded(maxStateEvents int64) error {
	return budgetExceeded(ErrStateEventLimitExceeded,
		fmt.Sprintf("state machine exceeded max events (%d events; raise %s to allow more), possible infinite loop",
			maxStateEvents, MaxStateEventsEnvVar))
}

// startDoAction registers a state's do behavior as running. Re-entering a
// state restarts its do behavior rather than resuming the abandoned one.
func (e *StateExecutor) startDoAction(state *ast.StateNode) {
	doBehaviors := e.behaviorsOf(state).Do
	if len(doBehaviors) == 0 {
		return
	}
	e.stopDoAction(state)
	e.doActions = append(e.doActions, &doAction{
		state:   state,
		pending: append([]lower.StateBehavior(nil), doBehaviors...),
	})
}

// behaviorsOf returns the lowered entry, do and exit behaviors of a state, and
// an empty set for a state lowering recorded none for.
func (e *StateExecutor) behaviorsOf(state *ast.StateNode) *lower.StateBehaviors {
	if behaviors, ok := e.graph.Behaviors[state]; ok && behaviors != nil {
		return behaviors
	}
	return &lower.StateBehaviors{}
}

// stopDoAction abandons whatever is left of a state's do behavior, which is
// what exiting the state does to it: a behavior paused on the clock ends there.
func (e *StateExecutor) stopDoAction(state *ast.StateNode) {
	kept := e.doActions[:0]
	for _, act := range e.doActions {
		if act.state != state {
			kept = append(kept, act)
			continue
		}
		if act.run != nil {
			e.endDoRun(act.run)
			act.run = nil
		}
	}
	for i := len(kept); i < len(e.doActions); i++ {
		e.doActions[i] = nil
	}
	e.doActions = kept
}

// endDoRun ends a do behavior an exit abandons; while a move a refused witness
// may undo is under way, the end waits for the move to be kept (see moveMark).
func (e *StateExecutor) endDoRun(run *doRun) {
	if e.moving != nil {
		e.moving.ended = append(e.moving.ended, run)
		return
	}
	run.end(e.ctx)
}

// runDoRound advances every running do behavior with an action due by one action
// and returns how many ran. One round is how concurrently active states share
// the machine: each performs one action before any performs its next, in an
// order the policy draws (entry order by default), reported as a choice where
// two or more are due. A behavior that waits on the clock or for a message
// pauses there and is a state's action for the round its wait ends in.
func (e *StateExecutor) runDoRound() (int, error) {
	if len(e.doActions) == 0 {
		return 0, nil
	}
	due := make([]*doAction, 0, len(e.doActions))
	for _, act := range e.doActions {
		if act.due(e.ctx) {
			due = append(due, act)
		}
	}

	ran := 0
	for len(due) > 0 {
		due = slices.DeleteFunc(due, func(act *doAction) bool { return !e.isRunningDoAction(act) })
		if len(due) == 0 {
			break
		}
		next, err := e.chooseDoAction(due)
		if err != nil {
			return ran, err
		}
		act := due[next]
		due = slices.Delete(due, next, next+1)
		if err := e.stepDoAction(act, func(run *doRun) (*doRun, error) { return run.resume(e.ctx) }); err != nil {
			return ran, err
		}
		ran++
	}
	return ran, e.settleDoActions()
}

// stepDoAction performs one action of a do behavior: the behavior under way goes
// on as told, else the next behavior begins.
func (e *StateExecutor) stepDoAction(act *doAction, goOn func(*doRun) (*doRun, error)) error {
	e.moved = true
	if e.trace() != nil {
		e.trace().RecordDoStep(act.state.Name)
	}
	var err error
	if act.run != nil {
		act.run, err = goOn(act.run)
	} else {
		behavior := act.pending[0]
		act.pending = act.pending[1:]
		act.run, err = e.startDoRun(behavior)
	}
	if err != nil {
		return fmt.Errorf("do action in state %s: %w", act.state.Name, err)
	}
	return nil
}

// doBehaviorsTaking lists the do behaviors the message being dispatched lets go on:
// those parked at an accept for it, except in a state a transition chosen for it
// leaves — the transition ending the state is the message's only taker there, as
// the innermost enabled transition is among transitions, while one moving between
// the state's own substates leaves its do behavior to go on. A port failing to
// resolve on the way is the error.
func (e *StateExecutor) doBehaviorsTaking(m Message, candidates []dispatchCandidate) ([]*doAction, error) {
	var taking []*doAction
	for _, act := range e.doActions {
		if act.run == nil || e.leftByChosen(act.state, candidates) {
			continue
		}
		accepted, err := act.run.acceptsMessage(m)
		if err != nil {
			return nil, fmt.Errorf("do action in state %s: %w", act.state.Name, err)
		}
		if accepted {
			taking = append(taking, act)
		}
	}
	return taking, nil
}

// leftByChosen reports whether firing the transition chosen for a candidate exits
// the state, itself or a state enclosing it; a firing that would fail counts as one.
func (e *StateExecutor) leftByChosen(state *ast.StateNode, candidates []dispatchCandidate) bool {
	for _, candidate := range candidates {
		exited, ok := e.exitedBy(candidate)
		if !ok {
			return true
		}
		for _, left := range exited {
			if e.isBelowOrEqual(state, left) {
				return true
			}
		}
	}
	return false
}

// exitedBy lists the states firing the candidate's chosen transition along its
// route exits, or may exit while the route is open at a choice, as fireFrom exits
// them; false where the firing would fail.
func (e *StateExecutor) exitedBy(candidate dispatchCandidate) ([]*ast.StateNode, bool) {
	trans, r := candidate.chosen, candidate.route
	if ps, ok := trans.Target.(*ast.PseudostateNode); ok && isSynchronizationTarget(ps) {
		return e.exitedBySynchronization(trans, ps, r)
	}
	if !r.settled() {
		return nil, false
	}
	if region := e.activeRegionOf(candidate.source); region != nil {
		source := e.activeConfig.regionStates[region]
		return e.mayExit(r, trans, source, func(target *ast.StateNode) []*ast.StateNode { return e.exitedInRegion(region, trans, target) })
	}
	origin := e.moveOrigin()
	return e.mayExit(r, trans, origin, func(target *ast.StateNode) []*ast.StateNode { return e.exitedByMove(origin, trans, target) })
}

// exitedInRegion lists the states a transition out of region's active state exits
// on its way to route, as fireTransitionInRegion exits them.
func (e *StateExecutor) exitedInRegion(region *ast.StateRegion, trans *lower.Transition, route *ast.StateNode) []*ast.StateNode {
	sourceRegion, targetRegion := e.regionMove(region, route)
	if targetRegion == nil {
		owner := e.graph.RegionOwner[region]
		if owner == nil {
			var exited []*ast.StateNode
			for _, top := range e.graph.TopRegions {
				if active, ok := e.activeConfig.regionStates[top]; ok {
					exited = append(exited, e.exitPath(active, nil, nil)...)
				}
			}
			return exited
		}
		lca := e.getLCA(owner, route)
		if lca == route {
			return e.regionsExitPath(route)
		}
		return e.exitPath(owner, lca, nil)
	}
	keep := e.regionKeep(targetRegion, trans, route)
	if sourceRegion == targetRegion {
		return e.regionExitPath(sourceRegion, keep)
	}
	return append(e.regionExitPath(sourceRegion, nil), e.regionExitPath(targetRegion, keep)...)
}

// exitedBySynchronization lists the states a transition into a fork, join or
// history exits, as fireTransition exits them; a join not yet synchronized exits none.
func (e *StateExecutor) exitedBySynchronization(trans *lower.Transition, ps *ast.PseudostateNode, r route) ([]*ast.StateNode, bool) {
	switch ps.Kind {
	case ast.PseudostateFork:
		plan, err := e.forkPlan(ps)
		if err != nil {
			return nil, false
		}
		return e.exitedForFork(trans, plan.Owner)
	case ast.PseudostateJoin:
		if !r.settled() {
			return nil, true
		}
		if _, err := e.joinSources(ps); err != nil {
			return nil, false
		}
		plan, err := e.joinPlan(ps)
		if err != nil {
			return nil, false
		}
		return e.joinExits(plan, trans, r)
	default:
		owner, err := e.historyOwner(ps)
		if err != nil {
			return nil, false
		}
		return e.exitedByMove(e.moveOrigin(), trans, owner), true
	}
}

// exitedForFork lists the states a transition into a fork whose branches enter
// owner's regions exits, as leaveForFork exits them; false where it would fail.
func (e *StateExecutor) exitedForFork(trans *lower.Transition, owner *ast.StateNode) ([]*ast.StateNode, bool) {
	source, _ := trans.Source.(*ast.StateNode)
	region := e.activeRegionOf(source)
	if region == nil {
		return e.exitedByMove(e.moveOrigin(), trans, owner), true
	}
	if _, targetRegion := e.regionMove(region, owner); targetRegion == nil && e.graph.RegionOwner[region] == nil {
		return nil, false
	}
	return e.exitedInRegion(region, trans, owner), true
}

// exitedByMove lists the states a move from current to target exits, as
// transitionToInto exits them.
func (e *StateExecutor) exitedByMove(current *ast.StateNode, trans *lower.Transition, target *ast.StateNode) []*ast.StateNode {
	return e.exitPath(current, e.moveBoundary(current, trans, target), nil)
}

// enteredByMove lists the states a move of the single active hierarchy from
// current to target enters, outermost first.
func (e *StateExecutor) enteredByMove(current *ast.StateNode, trans *lower.Transition, target *ast.StateNode) []*ast.StateNode {
	return e.descendantChain(e.moveBoundary(current, trans, target), target)
}

// enteredInRegion lists the states a transition out of region's active state
// enters on its way to target, outermost first, as moveInRegion enters them.
func (e *StateExecutor) enteredInRegion(region *ast.StateRegion, trans *lower.Transition, target *ast.StateNode) []*ast.StateNode {
	_, targetRegion := e.regionMove(region, target)
	if targetRegion == nil {
		owner := e.graph.RegionOwner[region]
		if owner == nil {
			return e.rootToLeaf(target)
		}
		return e.descendantChain(e.getLCA(owner, target), target)
	}
	return e.descendantChain(e.regionKeep(targetRegion, trans, target), target)
}

// resumeDoBehaviors lets the do behaviors taking the message being dispatched go
// on with it, before the transitions selected fire, and names the states whose
// behavior did. One an earlier behavior's step has already ended is skipped.
func (e *StateExecutor) resumeDoBehaviors(taking []*doAction, m Message) ([]string, error) {
	var resumed []string
	for _, act := range taking {
		if !e.isRunningDoAction(act) {
			continue
		}
		if err := e.stepDoAction(act, func(run *doRun) (*doRun, error) { return run.offer(e.ctx, m) }); err != nil {
			return resumed, err
		}
		resumed = append(resumed, doBehaviorDescription(act.state))
	}
	if len(resumed) == 0 {
		return nil, nil
	}
	return resumed, e.settleDoActions()
}

// doBehaviorDescription names a state's do behavior as decisions and dispatches report it.
func doBehaviorDescription(state *ast.StateNode) string {
	return "do behavior of state " + StateVertexName(state)
}

// settleDoActions drops the do behaviors that have finished and schedules the
// completion of their states; a do action whose state was exited is already gone.
func (e *StateExecutor) settleDoActions() error {
	finished := make([]*ast.StateNode, 0, len(e.doActions))
	kept := e.doActions[:0]
	for _, act := range e.doActions {
		if !act.finished() {
			kept = append(kept, act)
			continue
		}
		finished = append(finished, act.state)
	}
	for i := len(kept); i < len(e.doActions); i++ {
		e.doActions[i] = nil
	}
	e.doActions = kept

	// A state completes once its do behavior has finished and its body, where
	// it runs one, has reached `done`; completeIfDone schedules the latter case.
	for _, state := range finished {
		if e.bodyRunning(state) && !e.stateComplete(state) {
			continue
		}
		if err := e.scheduleCompletionTransitions(state); err != nil {
			return fmt.Errorf("schedule completion of state %s: %w", state.Name, err)
		}
	}
	return nil
}

// chooseDoAction resolves which of the do behaviors due in a round acts next: the
// policy draws the pick and, with several due, the choice is reported; a replay
// that cannot follow its witness here is its refusal, and none acts.
func (e *StateExecutor) chooseDoAction(due []*doAction) (int, error) {
	if len(due) < 2 {
		return 0, nil
	}
	states := make([]*ast.StateNode, len(due))
	for i, act := range due {
		states[i] = act.state
	}
	choice := e.regionOrderChoice("do round at t="+semantics.FormatReal(e.ctx.clock.now), states)
	if err := e.ctx.scheduling().refusal(); err != nil {
		return 0, err
	}
	e.ctx.noteChoice(choice)
	return choice.Taken, nil
}

// isRunningDoAction reports whether a do action is still registered, which it
// is not once its state has been exited.
func (e *StateExecutor) isRunningDoAction(act *doAction) bool {
	for _, running := range e.doActions {
		if running == act {
			return true
		}
	}
	return false
}

// hasRunningDoAction reports whether a state's do behavior is still running.
func (e *StateExecutor) hasRunningDoAction(state *ast.StateNode) bool {
	for _, act := range e.doActions {
		if act.state == state {
			return true
		}
	}
	return false
}

// SendSignal injects a signal event into the state machine.
// This is the primary API for driving state machines with external signals.
// The signal is enqueued and will be processed on the next ProcessNextEvent call.
func (e *StateExecutor) SendSignal(signalType string, args map[string]Value) {
	e.enqueueSignal(Message{SignalType: signalType, Payload: args})
}

// InvokeOperation injects a call event for the named operation. Transitions
// triggered by that operation fire; transitions triggered by another do not.
func (e *StateExecutor) InvokeOperation(operation string, args map[string]Value) {
	e.moved = true
	e.eventQueue.Push(Event{
		ID:        e.nextEventID,
		Type:      EventCall,
		Timestamp: e.ctx.clock.now,
		Payload:   Call{Operation: operation, Args: args},
	})
	e.nextEventID++
}

// enqueueSignal queues a message as an accept event, to fire immediately.
func (e *StateExecutor) enqueueSignal(msg Message) {
	e.moved = true
	e.eventQueue.Push(Event{
		ID:        e.nextEventID,
		Type:      EventAccept,
		Timestamp: e.ctx.clock.now,
		Payload:   msg,
	})
	e.nextEventID++
}

// deliverPendingSignal reports whether an event is due, first queueing a bus
// message this machine takes: one in flight is due now, so it goes ahead of a
// timer set for later rather than after the run has advanced to it. Messages
// this machine leaves, and one whose port fails to resolve, stay in flight.
func (e *StateExecutor) deliverPendingSignal() (bool, error) {
	var failed error
	msg, ok := e.ctx.TakeMessage(func(m Message) bool {
		if failed != nil {
			return false
		}
		takes, err := e.takesMessage(m)
		if err != nil {
			failed = err
			return false
		}
		return takes
	})
	if failed != nil {
		return false, failed
	}
	if !ok {
		return e.hasDueEvent(), nil
	}
	e.enqueueSignal(msg)
	return true, nil
}

// takesMessage reports whether this machine is the one to take a message in
// flight: one it can react to, unless its guards would drop it while a sibling
// machine of the same object would fire on or defer it.
func (e *StateExecutor) takesMessage(m Message) (bool, error) {
	if reacts, err := e.reactsTo(m); err != nil || !reacts {
		return reacts, err
	}
	return !e.yieldsTo(m), nil
}

// yieldsTo reports whether a machine the performer also exhibits, one that
// would fire on or defer the message where this one would only drop it, should
// take it instead. A guard error is left for the dispatch of the machine whose
// guard it is to report: this machine takes the message when its own guard
// fails, and leaves it when a sibling's does.
func (e *StateExecutor) yieldsTo(m Message) bool {
	siblings := e.siblingsAccepting(m)
	if len(siblings) == 0 {
		return false
	}
	if own, err := e.Decide(m); err != nil || own.Enabled() {
		return false
	}
	for _, sibling := range siblings {
		if theirs, err := sibling.Decide(m); err != nil || theirs.Enabled() {
			return true
		}
	}
	return false
}

// siblingsAccepting lists the other machines the performer exhibits whose active
// configuration accepts the message, in the order they were attached. One whose
// accept port fails to resolve is listed too: its own dispatch reports that.
func (e *StateExecutor) siblingsAccepting(m Message) []*StateExecutor {
	if e.self == nil {
		return nil
	}
	var siblings []*StateExecutor
	for _, behavior := range e.ctx.objectBehaviors {
		sibling := behavior.State
		if sibling == nil || sibling == e || behavior.Object != e.self {
			continue
		}
		if accepted, err := sibling.reactsTo(m); err != nil || accepted {
			siblings = append(siblings, sibling)
		}
	}
	return siblings
}

// reactsTo reports whether a message in flight is one this machine would act on:
// its configuration accepts or defers it, or a do behavior under way is parked at
// an accept for it, which dispatching the message lets go on with it.
func (e *StateExecutor) reactsTo(m Message) (bool, error) {
	if accepted, err := e.acceptableMessage(m); err != nil || accepted {
		return accepted, err
	}
	taking, err := e.doBehaviorsTaking(m, nil)
	return len(taking) > 0, err
}

// acceptableMessage reports whether a message in flight is one this machine can
// react to now: a transition out of the active configuration accepts it (one
// routed to a port only `via` that port), or the message reaches this machine —
// addressed to it, or at a port of the object performing it — and a state of the
// configuration defers it, to be held until a transition accepts it. Resolving
// the port a `via` names may materialize it, which can fail.
func (e *StateExecutor) acceptableMessage(m Message) (bool, error) {
	if accepted, err := e.acceptsSignal(m); err != nil || accepted {
		return accepted, err
	}
	return m.reaches(e.stateMachine.Name, m.Port, objectID(e.self)) && e.defersMessage(m), nil
}

// defersMessage reports whether the active configuration defers a message,
// whatever route it came by, as a trigger naming no port takes it.
func (e *StateExecutor) defersMessage(m Message) bool {
	event := Event{Type: EventAccept, Timestamp: e.ctx.clock.now, Payload: m}
	return e.defersEvent(&event)
}

// HasPendingSignal reports whether a signal this machine reacts to is in flight:
// one its configuration accepts or defers, or a do behavior under way is parked at
// an accept for. Such a signal is due now, unlike a queued event's timestamp: the
// next step delivers and dispatches it.
func (e *StateExecutor) HasPendingSignal() bool {
	return e.hasPendingSignal()
}

// AcceptsMessage reports whether the machine would take a message in flight: it
// reaches this machine and triggers a transition out of an active state, is
// deferred by one, or lets a do behavior parked at an accept go on. Whether a
// transition fires is decided by its guard; see Decide. Resolving the port a
// trigger accepts `via` may fail; a port it materializes on the way is discarded,
// as everything the preview builds.
func (e *StateExecutor) AcceptsMessage(m Message) (accepted bool, err error) {
	e.preview(func() { accepted, err = e.reactsTo(m) })
	return accepted, err
}

// preview runs fn as a probe of the context, which beginProbe undoes whole.
func (e *StateExecutor) preview(fn func()) {
	defer e.ctx.beginProbe()()
	fn()
}

// Decision is what dispatching a message now would do: the transitions that
// would fire on it, that the active state would defer it, or the do behaviors
// parked at an accept it lets go on.
type Decision struct {
	Fires    []string
	Deferred bool
	Resumes  []string
}

// Enabled reports whether dispatching the message would do something with it.
func (d Decision) Enabled() bool {
	return len(d.Fires) > 0 || d.Deferred || len(d.Resumes) > 0
}

// Decide decides a message as the step dispatching it would — the payload bound,
// the guards evaluated against the data as it stands — without taking it. The
// machine, its data, its budget and the message are left as they were found: a
// value a guard derives on the way is derived again when dispatch reads it, and an
// object materialized on the way — to bind the payload, or by a guard reaching an
// occurrence not yet built — starts its behaviors as under dispatch, so the guard
// reads what they make of it, then is discarded along with them and anything they
// sent. So is a port materialized to tell whether the message reaches the machine
// at all. A payload or guard error is returned. Among several enabled transitions
// it names the one this machine's own run would fire, its scheduler left in place.
// A do behavior parked at an accept the message would let go on is named too.
func (e *StateExecutor) Decide(m Message) (decision Decision, err error) {
	defer e.ctx.previewExecutorRun(&e.driven)()
	e.preview(func() { decision, err = e.decide(m) })
	return decision, err
}

// decide is Decide under the preview that discards what it builds. It selects the
// message's takers as broadcastEvent does, so the two agree.
func (e *StateExecutor) decide(m Message) (Decision, error) {
	event := Event{Type: EventAccept, Timestamp: e.ctx.clock.now, Payload: m}
	accepted, err := e.acceptableMessage(m)
	if err != nil {
		return Decision{}, err
	}
	var candidates []dispatchCandidate
	if accepted {
		selected, err := e.selectTransitions(&event)
		if err != nil {
			return Decision{}, err
		}
		if candidates, err = e.chooseTransitions(selected, &event); err != nil {
			return Decision{}, err
		}
	}
	taking, err := e.doBehaviorsTaking(m, candidates)
	if err != nil {
		return Decision{}, err
	}
	var decision Decision
	for _, candidate := range candidates {
		decision.Fires = append(decision.Fires, transitionDescription(candidate.chosen))
	}
	for _, act := range taking {
		decision.Resumes = append(decision.Resumes, doBehaviorDescription(act.state))
	}
	if accepted && !decision.Enabled() {
		decision.Deferred = e.defersEvent(&event)
	}
	return decision, nil
}

// Performer is the object this machine is performed by, nil for none.
func (e *StateExecutor) Performer() *Instance {
	return e.self
}

// hasPendingSignal reports whether the next step would act on a message in
// flight, without consuming it: deliver one, or report an accept port that
// fails to resolve.
func (e *StateExecutor) hasPendingSignal() bool {
	for _, msg := range e.ctx.PendingMessages() {
		if takes, err := e.takesMessage(msg); err != nil || takes {
			return true
		}
	}
	return false
}

// acceptsSignal reports whether any transition out of the active configuration,
// or out of a composite state enclosing it, is triggered by this signal.
func (e *StateExecutor) acceptsSignal(msg Message) (bool, error) {
	for _, leaf := range e.activeStates() {
		for _, state := range e.getParentChain(leaf) {
			if accepted, err := e.acceptsSignalFrom(state, msg); err != nil || accepted {
				return accepted, err
			}
		}
	}
	return false, nil
}

// acceptsSignalFrom reports whether a transition out of one state is triggered
// by this message's signal. Only a message of the right signal resolves the
// transition's `via` port; a failure to do so is returned.
func (e *StateExecutor) acceptsSignalFrom(state *ast.StateNode, msg Message) (bool, error) {
	for _, trans := range e.graph.Transitions[state] {
		accept, ok := trans.Trigger.(*ast.AcceptEvent)
		if !ok || !e.triggerSignalMatches(accept, trans.Scope, msg) {
			continue
		}
		reaches, err := e.ctx.messageReaches(msg, e.stateMachine.Name, trans.Via, e.self)
		if err != nil || reaches {
			return reaches, err
		}
	}
	return false, nil
}

// triggerSignalMatches reports whether the message carries the signal an
// accept trigger names, by type or by the event it subsets.
func (e *StateExecutor) triggerSignalMatches(accept *ast.AcceptEvent, scope *symbols.Scope, msg Message) bool {
	if typed := ast.AsQualifiedName(accept.SignalType); typed != nil && len(typed.Parts) > 0 {
		return e.ctx.messageMatches(msg, typed, scope)
	}
	return lower.FeaturePath(accept.Subsets) != "" && e.triggerEval(scope).carriesEvent(msg, accept.Subsets)
}

// triggerEval evaluates what a trigger declared in scope names, over the
// machine's data.
func (e *StateExecutor) triggerEval(scope *symbols.Scope) *EvalContext {
	ec := NewEvalContextIn(e.ctx, scope, e.self)
	ec.inBehaviorBody = true
	ec.Push(e.stateData)
	return ec
}

// activeStates returns the states currently active, in region declaration
// order: one per region for a composite configuration, otherwise the single
// active state.
func (e *StateExecutor) activeStates() []*ast.StateNode {
	if len(e.activeConfig.regionStates) > 0 {
		return e.orderedRegionStates()
	}
	if current := e.getCurrentState(); current != nil {
		return []*ast.StateNode{current}
	}
	return nil
}

// initialize sets current state to initial state and enters it.
func (e *StateExecutor) initialize() (err error) {
	defer e.ctx.beginExecutorRun(&e.driven)()
	defer e.completedWhole(&err)
	defer e.unfireOnError(len(e.fired), &err)
	e.ctx.beginPerformanceLife(e.occurrence, e.ctx.newActivation())

	// A machine without orthogonal regions of its own starts in the state its
	// body's entry transitions choose, then in that state's own start, and so on.
	if len(e.graph.TopRegions) == 0 {
		if len(e.graph.StartOf(nil)) == 0 {
			return fmt.Errorf("%w in state machine %s", ErrNoInitialState, e.stateMachine.Name)
		}
		if err := e.enterMachine(); err != nil {
			return fmt.Errorf("enter state machine: %w", err)
		}
		start, err := e.startIn(nil)
		if err != nil {
			return err
		}

		e.setCurrentState(start)
		e.state = StateRunning
		e.stateStack = e.rootToLeaf(start)
		for _, state := range e.stateStack {
			if err := e.enterState(state); err != nil {
				return fmt.Errorf("enter state %s: %w", state.Name, err)
			}
		}
		leaf, err := e.enterStartOf(start)
		if err != nil {
			return err
		}
		e.stateStack = e.rootToLeaf(leaf)

		if err := e.scheduleTransitionEvents(); err != nil {
			return fmt.Errorf("schedule events: %w", err)
		}
		// An entry transition choosing `done` completes the machine as it starts.
		if err := e.completeIfDone(leaf); err != nil {
			return fmt.Errorf("complete state machine: %w", err)
		}
		return nil
	}

	if e.graph.Machine != nil {
		if err := e.enterMachine(); err != nil {
			return fmt.Errorf("enter state machine: %w", err)
		}
	}

	e.state = StateRunning
	e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode)
	e.activeConfig.simpleState = nil

	// Enter the initial state of each of the machine's own regions, in declaration
	// order: the order they are entered in is observable.
	if err := e.enterRegionsInto(nil, e.graph.TopRegions, nil); err != nil {
		return err
	}

	// Schedule events for outgoing transitions in all regions
	if err := e.scheduleTransitionEvents(); err != nil {
		return fmt.Errorf("schedule events: %w", err)
	}

	// Every region starting in `done` completes the machine as it starts.
	for _, region := range e.graph.TopRegions {
		if err := e.completeIfDone(e.activeConfig.regionStates[region]); err != nil {
			return fmt.Errorf("complete state machine: %w", err)
		}
		if e.state.Ended() {
			break
		}
	}
	return nil
}

// enterMachine runs the machine's own entry behaviors and starts its do behavior.
func (e *StateExecutor) enterMachine() error {
	behaviors := e.behaviorsOf(e.graph.Machine)
	for _, behavior := range behaviors.Entry {
		if err := e.executeBehavior(behavior); err != nil {
			return fmt.Errorf("entry action: %w", err)
		}
	}
	e.startDoAction(e.graph.Machine)
	return nil
}

// startIn chooses the state owner's body starts in: the target of the first
// transition out of its entry action whose guard holds, in declaration order.
// It is nil when the body declares none; when none holds, that is an error.
func (e *StateExecutor) startIn(owner ast.Node) (*ast.StateNode, error) {
	transitions := e.graph.StartOf(owner)
	for _, entry := range transitions {
		holds, err := e.entryGuardHolds(owner, entry)
		if err != nil {
			return nil, err
		}
		if holds {
			if entry.Decl != nil {
				e.fired = append(e.fired, FiredTransition{Decl: entry.Decl, Target: entry.Target, Owner: e.graph.EntryOwner(owner)})
			}
			return entry.Target, nil
		}
	}
	if len(transitions) > 0 {
		return nil, fmt.Errorf("%w: %s declares %d transitions out of its entry action and the guard of none holds",
			ErrNoEntryTransitionHolds, e.describeBody(owner), len(transitions))
	}
	return nil, nil
}

// entryGuardHolds evaluates an entry transition's guard in the body it is
// written in, reading the attributes of the state that owns that body.
func (e *StateExecutor) entryGuardHolds(owner ast.Node, entry *lower.EntryTransition) (bool, error) {
	if entry.Guard == nil {
		return true, nil
	}
	val, err := e.evalStepOf(e.bodyState(owner), entry.Guard, entry.Scope)
	if err != nil {
		return false, fmt.Errorf("eval guard of the entry transition into %s: %w", entry.Target.Name, err)
	}
	if val.Kind != ValConst || val.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("guard of the entry transition into %s must be boolean, got %v", entry.Target.Name, val.Kind)
	}
	return val.Const.Bool, nil
}

// bodyState is the state whose attributes a body's entry transitions read: the
// state itself, a region's owner, or nil for the machine's own body.
func (e *StateExecutor) bodyState(owner ast.Node) ast.Node {
	switch body := owner.(type) {
	case *ast.StateNode:
		return body
	case *ast.StateRegion:
		if state := e.graph.RegionOwner[body]; state != nil {
			return state
		}
	}
	return nil
}

// describeBody names the body an entry transition is written in for a diagnostic.
func (e *StateExecutor) describeBody(owner ast.Node) string {
	switch body := owner.(type) {
	case *ast.StateNode:
		return fmt.Sprintf("state %s", body.Name)
	case *ast.StateRegion:
		if state := e.graph.RegionOwner[body]; state != nil {
			return fmt.Sprintf("region %s of state %s", body.Name, state.Name)
		}
		return fmt.Sprintf("region %s", body.Name)
	}
	return fmt.Sprintf("state machine %s", e.stateMachine.Name)
}

// enterStartOf enters the state a just-entered state's body starts in, and that
// state's own start below it, returning the innermost state entered. A state
// whose substates are orthogonal regions has entered them already.
func (e *StateExecutor) enterStartOf(state *ast.StateNode) (*ast.StateNode, error) {
	leaf := state
	for {
		if _, orthogonal := e.graph.CompositeStates[leaf]; orthogonal {
			return leaf, nil
		}
		start, err := e.startIn(leaf)
		if err != nil {
			return nil, err
		}
		if start == nil {
			return leaf, nil
		}
		for _, descendant := range e.descendantChain(leaf, start) {
			if err := e.enterState(descendant); err != nil {
				return nil, fmt.Errorf("enter state %s: %w", descendant.Name, err)
			}
		}
		leaf = start
	}
}

// exitMachine stops the machine's own do behavior and runs its exit behaviors
// once, when the machine completes.
func (e *StateExecutor) exitMachine() error {
	if e.graph.Machine == nil || e.machineExited {
		return nil
	}
	e.machineExited = true
	e.stopDoAction(e.graph.Machine)
	for _, behavior := range e.behaviorsOf(e.graph.Machine).Exit {
		if err := e.executeBehavior(behavior); err != nil {
			return fmt.Errorf("exit action: %w", err)
		}
	}
	return nil
}

// descendantChain returns the states from ancestor's child down to leaf,
// outermost first, excluding ancestor itself. A nil ancestor returns the whole
// chain from leaf's outermost ancestor down; leaf alone is returned when
// ancestor is neither nil nor one of its ancestors.
func (e *StateExecutor) descendantChain(ancestor, leaf *ast.StateNode) []*ast.StateNode {
	if leaf == nil {
		return nil
	}
	chain := e.getParentChain(leaf) // leaf .. root
	if ancestor == nil {
		return e.rootToLeaf(leaf)
	}
	for i, state := range chain {
		if state == ancestor {
			descendants := make([]*ast.StateNode, 0, i)
			for j := i - 1; j >= 0; j-- {
				descendants = append(descendants, chain[j])
			}
			return descendants
		}
	}
	return []*ast.StateNode{leaf}
}

// enterState executes entry behaviors when entering a state.
func (e *StateExecutor) enterState(state *ast.StateNode) error {
	return e.enterStateInto(state, nil)
}

// enterStateInto enters state, starting each of its orthogonal regions at
// branches[region] where branches names one and at the region's initial state
// otherwise, bypassing that initial state's entry and do behaviors.
func (e *StateExecutor) enterStateInto(state *ast.StateNode, branches map[*ast.StateRegion]*ast.StateNode) error {
	if state == nil {
		return nil
	}
	if err := e.activateState(state); err != nil {
		return err
	}
	if regions, isComposite := e.graph.CompositeStates[state]; isComposite {
		if err := e.enterRegionsInto(state, regions, branches); err != nil {
			return err
		}
	}

	// The do behavior runs while the state is active, interleaved with the do
	// behaviors of the states active alongside it, rather than at entry.
	e.startDoAction(state)

	// Don't schedule transitions here - let the caller decide when to schedule
	// This prevents double-scheduling in region transitions

	return nil
}

// activateState makes state active and runs its entry behaviors, leaving its
// orthogonal regions, if any, to be entered and its do behavior to be started.
func (e *StateExecutor) activateState(state *ast.StateNode) error {
	if !e.activatedAhead(state) {
		if err := e.performEntry(state); err != nil {
			return err
		}
	}

	// A composite of orthogonal regions is represented by their active states;
	// entries of other composites' regions are left alone. A simple state is the
	// single active state only outside any region.
	if _, isComposite := e.graph.CompositeStates[state]; isComposite {
		e.activeConfig.simpleState = nil
	} else if len(e.activeConfig.regionStates) == 0 {
		e.activeConfig.simpleState = state
	}
	return nil
}

// performEntry records the entry of state and performs its entry behaviors.
func (e *StateExecutor) performEntry(state *ast.StateNode) error {
	// Change watches are created fresh per activation, so a condition that stayed
	// true rises again; the firing transition keeps its latch so the entry it
	// caused does not re-enable it.
	for _, trans := range e.graph.Transitions[state] {
		if trans != e.firingChange {
			delete(e.changeFired, trans)
			if e.changeRearmed != nil {
				e.changeRearmed[trans] = true
			}
		}
	}

	if e.breakpointNodes[state] && e.breakpointHit == nil {
		e.breakpointHit = state
	}

	if !e.graph.HiddenStates[state] {
		// Track state visit
		e.stateVisits = append(e.stateVisits, state.Name)

		// Record trace
		if e.trace() != nil {
			e.trace().RecordStateEntry(state.Name, len(e.behaviorsOf(state).Entry) > 0)
		}
	}

	// Execute entry actions
	for _, behavior := range e.behaviorsOf(state).Entry {
		if err := e.executeBehavior(behavior); err != nil {
			return fmt.Errorf("entry action: %w", err)
		}
	}
	return nil
}

// exitState executes exit behaviors when leaving a state.
func (e *StateExecutor) exitState(state *ast.StateNode) error {
	if state == nil {
		return nil
	}

	// A state's timers are destroyed when it is left: the event one already queued
	// is withdrawn, so re-entering the state times a fresh interval.
	timed := make(map[*lower.Transition]bool)
	for _, trans := range e.graph.Transitions[state] {
		delete(e.timerScheduled, trans)
		if _, isTime := trans.Trigger.(*ast.TimeEvent); isTime {
			timed[trans] = true
		}
	}
	if len(timed) > 0 {
		e.eventQueue.Withdraw(func(event Event) bool {
			trans, ok := event.Payload.(*lower.Transition)
			return ok && timed[trans]
		})
	}

	// Check if this state is a composite state with regions
	regions, isComposite := e.graph.CompositeStates[state]

	// The active configuration of every composite state lives in one map, so only
	// this state's own regions may be recorded or torn down here: touching the
	// whole map would stop the regions of an enclosing composite state too.
	active := make(map[*ast.StateRegion]*ast.StateNode, len(regions))
	for _, region := range regions {
		if regionState, isActive := e.activeConfig.regionStates[region]; isActive {
			active[region] = regionState
		}
	}

	// Remember the configuration being left, so a history pseudostate owned by
	// this state or by its parent can restore it.
	for region, regionState := range active {
		if regionState != state {
			e.recordRegionHistory(region, regionState)
		}
	}
	if parent := e.graph.ParentState[state]; parent != nil {
		e.recordChildHistory(parent, state)
	} else if e.graph.Machine != nil && e.graph.RegionOf[state] == nil && !e.graph.HiddenStates[state] {
		e.recordChildHistory(e.graph.Machine, state)
	}

	// Exit the active state of each of this state's regions, in declaration order.
	if isComposite {
		for _, region := range regions {
			regionState, isActive := active[region]
			if !isActive {
				continue
			}
			// Clear the entry first: the recursive exit walks the same map, and the
			// region still pointing at regionState would exit it a second time.
			delete(e.activeConfig.regionStates, region)
			// A region's active state may be nested below the region, so exit it and
			// the states between it and this one: their exit behaviors run too.
			for current := regionState; current != nil && current != state; current = e.graph.ParentState[current] {
				if err := e.exitState(current); err != nil {
					return fmt.Errorf("exit region state: %w", err)
				}
			}
		}
	}

	if e.leftAhead[state] {
		e.activeConfig.simpleState = nil
		return nil
	}
	if e.exitingAhead {
		e.leftAhead[state] = true
	}

	// Leaving the state abandons whatever is left of its do behavior, before the
	// exit behavior runs.
	e.stopDoAction(state)

	// Record trace
	if !e.graph.HiddenStates[state] && e.trace() != nil {
		e.trace().RecordStateExit(state.Name, len(e.behaviorsOf(state).Exit) > 0)
	}

	// Execute exit actions
	for _, behavior := range e.behaviorsOf(state).Exit {
		if err := e.executeBehavior(behavior); err != nil {
			return fmt.Errorf("exit action: %w", err)
		}
	}

	// Clear simple state
	e.activeConfig.simpleState = nil

	return nil
}

// invokeNested performs an action from a state's entry/exit/effect behavior,
// passing state data in through the callee's input parameters and merging its
// output parameters back into state data.
func (e *StateExecutor) invokeNested(inv actionInvocation) error {
	_, outputs, err := invokeAction(e.ctx, e.stateMachine.Scope, inv, e.stateData, e.self)
	if err != nil {
		return err
	}
	for _, name := range slices.Sorted(maps.Keys(outputs)) {
		if err := e.writeStateValue(name, outputs[name]); err != nil {
			return err
		}
	}
	return nil
}

// writeStateValue writes a value a performed action returned to the machine's
// attribute of that name, or to its state data where it declares none.
func (e *StateExecutor) writeStateValue(name string, value Value) error {
	e.moved = true
	if e.declaresAttribute(name) {
		return e.assignAttribute(name, value)
	}
	e.stateData[name] = value
	return nil
}

// stateActionName names a state action in diagnostics, falling back to what it
// references when the usage is anonymous (`entry a.b;`). This is deliberately
// not ast.EffectiveName: a diagnostic names the whole path written, `a.b`,
// where the effective name is just the feature named, `b`.
func stateActionName(u *ast.Usage) string {
	if u.Ident.Name != "" {
		return u.Ident.Name
	}
	for _, rel := range u.Relationships {
		if rel.Kind != ast.RelReferences && rel.Kind != ast.RelTyping {
			continue
		}
		switch target := rel.Target.(type) {
		case *ast.QualifiedName:
			return qualifiedNameText(target)
		case *ast.FeatureChainExpr:
			return "feature chain " + qualifiedNameText(target.Member)
		}
	}
	return "<anonymous>"
}

// --- Public accessor methods for REPL debugging ---

// CurrentState returns the current active state node.
func (e *StateExecutor) CurrentState() ast.Node {
	// For backward compatibility: return simple state if no regions active
	if e.activeConfig.simpleState != nil {
		return e.activeConfig.simpleState
	}
	// If multi-region, return nil (caller should check regions individually)
	return nil
}

// ActiveStates returns the machine's active state configuration: the single
// active state, or one state per orthogonal region, in declaration order.
func (e *StateExecutor) ActiveStates() []*ast.StateNode {
	return e.activeStates()
}

// GetStateVisits returns the ordered list of visited state names.
func (e *StateExecutor) GetStateVisits() []string {
	return e.stateVisits
}

// FinalStateName is the active configuration as a conformance case writes it:
// the orthogonal regions' active states joined by "+" in region name order,
// regions of one name in declaration order.
func (e *StateExecutor) FinalStateName() string {
	if regions := e.orderedActiveRegions(); len(regions) > 0 {
		sort.SliceStable(regions, func(i, j int) bool { return regions[i].Name < regions[j].Name })
		names := make([]string, len(regions))
		for i, region := range regions {
			names[i] = e.activeConfig.regionStates[region].Name
		}
		return strings.Join(names, "+")
	}
	if state, ok := e.CurrentState().(*ast.StateNode); ok {
		return state.Name
	}
	return ""
}

// getCurrentState returns the active simple state (nil if multi-region).
func (e *StateExecutor) getCurrentState() *ast.StateNode {
	return e.activeConfig.simpleState
}

// setCurrentState sets the active simple state (for non-region states).
func (e *StateExecutor) setCurrentState(state *ast.StateNode) {
	e.activeConfig.simpleState = state
	e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode) // Clear regions
}

// StateStack returns a copy of the state stack (active configuration).
func (e *StateExecutor) StateStack() []*ast.StateNode {
	stack := make([]*ast.StateNode, len(e.stateStack))
	copy(stack, e.stateStack)
	return stack
}

// StateData returns a copy of state machine local data, together with the
// attributes each state owns under that state's path (`nested.hits`), which two
// usages of one state definition hold separately.
func (e *StateExecutor) StateData() map[string]Value {
	data := make(map[string]Value, len(e.stateData))
	for k, v := range e.stateData {
		data[k] = v
	}
	for state, attrs := range e.stateAttrs {
		prefix := e.statePath(state) + "."
		for name, value := range attrs {
			data[prefix+name] = value
		}
	}
	return data
}

// statePath is a state's name qualified by the states enclosing it.
func (e *StateExecutor) statePath(state *ast.StateNode) string {
	chain := e.getParentChain(state)
	parts := make([]string, 0, len(chain))
	for i := len(chain) - 1; i >= 0; i-- {
		parts = append(parts, chain[i].Name)
	}
	return strings.Join(parts, ".")
}

// trace returns the recorder this executor's context is attached to, so turning
// reporting on or off reaches an execution already under way.
func (e *StateExecutor) trace() *TraceRecorder {
	return e.ctx.trace
}

// SetTrace sets the trace recorder for this executor and the context it
// evaluates in.
func (e *StateExecutor) SetTrace(trace *TraceRecorder) {
	e.ctx.SetTrace(trace)
}

// EventQueue returns the event queue (not copied - read-only access).
func (e *StateExecutor) EventQueue() *EventQueue {
	return e.eventQueue
}

// DeferredEvents returns the events the active configuration holds deferred, in
// the order they were deferred; they return to the queue once no active state
// defers them.
func (e *StateExecutor) DeferredEvents() []Event {
	return append([]Event(nil), e.deferred...)
}

// CurrentTime returns the current instant of the clock this machine shares with
// every executor of its context, in seconds.
func (e *StateExecutor) CurrentTime() float64 {
	return e.ctx.clock.now
}

// LastEventAt returns the instant the machine last dispatched an event at, zero
// before it dispatched any.
func (e *StateExecutor) LastEventAt() float64 {
	return e.lastEventAt
}

// NextWait returns the instant the machine's earliest wait — a timer, or a do
// behavior paused on the clock — comes due at, false when none is set for later
// than the clock's instant.
func (e *StateExecutor) NextWait() (float64, bool) {
	waits := e.clockWaits()
	if len(waits) == 0 {
		return 0, false
	}
	return waits[0].Due, true
}

// Release withdraws a machine its driver is done with from the clock, ending the
// do behaviors paused on it; safe to call more than once.
func (e *StateExecutor) Release() {
	for _, act := range e.doActions {
		if act.run != nil {
			act.run.end(e.ctx)
			act.run = nil
		}
	}
	e.ctx.clock.detach(e)
}

// Resume returns a machine suspended at quiescence to running, so a driver that
// makes work available — advancing time, or delivering an event — can step it
// again. A completed or failed machine is left as it is; one paused on its
// completion vertex completes as the next step.
func (e *StateExecutor) Resume() bool {
	if e.state != StateSuspended {
		return false
	}
	e.state, e.pausedAt = StateRunning, nil
	return true
}

// SetBreakpointAt pauses a run once the dispatch entering the state, or passing
// through the pseudostate, completes, even one leaving the state again; what is
// still due then waits until the machine resumes. Other nodes are ignored.
func (e *StateExecutor) SetBreakpointAt(vertex ast.Node) {
	switch v := vertex.(type) {
	case *ast.StateNode:
		if v != nil {
			e.breakpointNodes[v] = true
		}
	case *ast.PseudostateNode:
		if v != nil {
			e.breakpointNodes[v] = true
		}
	}
}

// ClearBreakpoints removes every breakpoint; a pause already reached stands.
func (e *StateExecutor) ClearBreakpoints() {
	e.breakpointNodes = make(map[ast.Node]bool)
}

// PausedAt is the breakpoint state or pseudostate the last run paused at, nil
// when none did.
func (e *StateExecutor) PausedAt() ast.Node {
	return e.pausedAt
}

// CompletionDue reports a machine paused on its completion vertex by a
// breakpoint: entering it completed the machine, which the next step finishes.
func (e *StateExecutor) CompletionDue() bool {
	return e.completionDue
}

// Suspend parks a running machine back at quiescence, for a driver that resumed
// it, found nothing to do and must not report it as running.
func (e *StateExecutor) Suspend() bool {
	if e.state != StateRunning {
		return false
	}
	e.state = StateSuspended
	return true
}

// State returns current execution state.
func (e *StateExecutor) State() ExecutionState {
	return e.state
}

// StateMachineSymbol returns the state machine being executed.
func (e *StateExecutor) StateMachineSymbol() *symbols.Symbol {
	return e.stateMachine
}

// Graph is the lowered state graph the executor runs, which a debugger reads to
// place the active configuration and the transitions it fired in a rendering.
func (e *StateExecutor) Graph() *lower.StateGraph {
	return e.graph
}

// ProcessNextEvent processes the next event from the queue (for REPL stepping).
// It is the same step RunToCompletion repeats: every active state's do behavior
// advances by one action, then the next event is dispatched. Advancing the do
// behaviors is progress in itself, so a step that ran one and found no event to
// dispatch succeeds — the completion transition it enables is queued next.
// With nothing due but a timer running, the shared clock advances to the earliest
// wait (running whatever else is due there) until this machine's next event is due.
func (e *StateExecutor) ProcessNextEvent() (err error) {
	defer e.ctx.beginExecutorRun(&e.driven)()
	defer e.completedWhole(&err)

	e.lastDispatch = nil
	if e.completionDue {
		return e.completeMachine()
	}
	var progress dueProgress
	for {
		ran, err := e.runDoRound()
		if err != nil {
			return err
		}
		// A condition risen in the do round is taken here, as RunToCompletion does.
		fired, err := e.pollChangeEvents()
		if err != nil {
			return fmt.Errorf("poll change conditions: %w", err)
		}
		if fired {
			return nil
		}
		// A signal sent by a behavior sharing this context is dispatched by the same
		// step RunToCompletion takes, so stepping and running agree.
		delivered, err := e.deliverPendingSignal()
		if err != nil {
			return err
		}
		if !delivered && ran > 0 {
			return nil
		}
		if _, waiting := e.NextWait(); delivered || !waiting {
			return e.processNextEvent()
		}
		// Polled and found nothing: settled until another executor gets somewhere.
		progress.settle(e)
		if err := e.awaitClock(&progress); err != nil {
			return err
		}
	}
}

// completedWhole makes a call of its own run that completed the machine return
// the refusal of the witness moves left over, when the call itself did not fail.
func (e *StateExecutor) completedWhole(err *error) {
	e.endedByOccurrence(err)
	if *err == nil && e.state.Ended() {
		*err = e.ctx.endedWhole(&e.driven)
	}
}

// awaitClock runs what is due, then moves the clock to the earliest wait, until this
// machine's turn comes (work due, or its change condition to poll) or no wait is left.
func (e *StateExecutor) awaitClock(progress *dueProgress) error {
	for {
		picked, err := e.ctx.runDue(e, progress)
		if err != nil {
			return err
		}
		if picked || e.dueWork() || !e.ctx.advanceToNextDue(progress) {
			return nil
		}
	}
}

// hasDueEvent reports whether an event the run may dispatch at the current
// instant is queued: one set for later is not yet due.
func (e *StateExecutor) hasDueEvent() bool {
	return e.eventQueue.Len() > 0 && e.eventQueue.Peek().Timestamp <= e.ctx.clock.now
}

// HasDueEvent reports whether an event due at the current instant is queued,
// which a run holding time where it is dispatches.
func (e *StateExecutor) HasDueEvent() bool {
	return e.hasDueEvent()
}

// HasPendingWork reports whether stepping the machine can still make progress:
// an event is queued, a signal this machine accepts is in flight, or a state's
// do behavior has actions left to run.
func (e *StateExecutor) HasPendingWork() bool {
	return e.completionDue || e.eventQueue.Len() > 0 || len(e.doActions) > 0 || e.hasPendingSignal()
}

// RunDoRound advances every active state's do behavior by one action, without
// dispatching any event, and reports how many actions ran.
func (e *StateExecutor) RunDoRound() (ran int, err error) {
	defer e.ctx.beginExecutorRun(&e.driven)()
	defer e.completedWhole(&err)

	return e.runDoRound()
}

// HasPendingDoWork reports whether some active state's do behavior still has an
// action to run. Such work is due now, unlike a queued event's timestamp.
func (e *StateExecutor) HasPendingDoWork() bool {
	for _, act := range e.doActions {
		if act.due(e.ctx) {
			return true
		}
	}
	return false
}
