package runtime

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// imagedBehavior is one object's execution of a behavior its type binds, by value.
// The lowered graph is kept as is: lowered IR is derived from the shared, frozen
// declarations and never written once lowered, so every context reads one copy.
type imagedBehavior struct {
	object    int64
	attached  int // position among the behaviors of the context imaged
	member    *symbols.Symbol
	binding   int
	name      string
	kind      lower.ClassifierBehaviorKind
	onClock   bool
	err       error
	typeBound bool
	deferred  *classifierBehaviorDecl
	action    *imagedAction
	state     *imagedState
}

// imagedAction is an action executor's state by value, its frames by position.
type imagedAction struct {
	graph             *lower.ActionGraph
	tokens            []Token
	tokenFrames       []int
	state             ExecutionState
	nextTokenID       int64
	nextRepetitionID  repetitionGroupID
	stepCount         int
	sweep, sweeps     uint64
	inputs            map[string]Value
	pausedAt          breakpointStop
	released          bool
	pauses            int64
	steps, stepsSpent int64
	inRun, moved      bool
	awaiting          int
	breakpoints       map[string]bool
	breakpointNodes   []NodeBreakpoint
	firedBreakpoints  map[breakpointVisit]bool
	traversals        []Traversal
	traversalBase     int
	keepTraversals    bool
	run               int
	dynamics          *stateSpaceRun
	frames            []imagedFrame
}

// imagedFrame is one performance's state by value, the frames it points at by position.
type imagedFrame struct {
	saved      actionFrame
	parent     int
	locals     []map[string]Value
	localCells [][]imagedBodyCell
	data       map[string]Value
	cells      []imagedBodyCell
	outer      []imagedOuter
	subactions map[ast.Node]int
	repeats    map[repetitionGroupID]imagedRepetition
	pending    map[ast.Node]map[string][]Value
	held       map[ast.Node]map[string][]nodeObject
	staged     map[ast.Node]map[string][]imagedStaged
	nested     map[ast.Node][]nestedDelivery
}

// imagedBodyCell stores a body binding's value and tracking state for an image.
type imagedBodyCell struct {
	name        string
	value       ast.Node
	scope       *symbols.Scope
	masked      string
	written     bool
	frozen      bool
	visible     []map[string]bool
	limitFrames bool
}

type imagedRepetition struct {
	node      ast.Node
	remaining int64
	live      []int
}

// imagedStaged is a staged streaming write, its source performance by position.
type imagedStaged struct {
	source int
	pin    string
	flow   ast.Node
	at     int
}

// imagedOuter is one level of bindings around a performance, its performance by position.
type imagedOuter struct {
	vars      map[string]Value
	aliases   map[string]string
	perf      int
	performed *symbols.Symbol
	run       int64
	merged    []*symbols.Symbol
}

// imagedState is a state executor's state by value.
type imagedState struct {
	graph              *lower.StateGraph
	state              ExecutionState
	activeConfig       *StateConfiguration
	nextEventID        int64
	events             []Event
	stateData          map[string]Value
	stateCells         []imagedBodyCell
	stateAttrs         map[*ast.StateNode]map[string]Value
	stateAttrCells     map[*ast.StateNode][]imagedBodyCell
	stateVisits        []string
	stateStack         []*ast.StateNode
	fired              []FiredTransition
	firedBase          int
	keepFired          bool
	breakpointNodes    map[ast.Node]bool
	breakpointHit      *ast.StateNode
	pausedAt           ast.Node
	completionDue      bool
	history            map[*ast.StateNode]historyRecord
	joinArrived        map[*ast.PseudostateNode][]*lower.Transition
	lastDispatch       *Dispatch
	lastEventAt        float64
	doActions          []doActionCapture
	machineExited      bool
	run                int
	inRun, moved       bool
	timerScheduled     map[*lower.Transition]bool
	timeTriggerVerdict map[*lower.Transition]error
	changeFired        map[*lower.Transition]bool
	changeObserved     map[*lower.Transition]bool
	changePending      map[*lower.Transition]bool
	firingChange       *lower.Transition
	firingNotes        []RunNote
	changeRearmed      map[*lower.Transition]bool
	changeWaits        []changeWait
	held               []heldEntry
	entering           map[*ast.StateNode]bool
	enteringMachine    bool
	activeAtEntry      map[*ast.StateNode]bool
	pendingCall        *pendingCall
}

// behavior takes one behavior's execution.
func (t *imaging) behavior(b *ObjectBehavior) error {
	img := imagedBehavior{
		object: b.Object.ID, attached: slices.Index(t.ctx.objectBehaviors, b),
		member: b.member, binding: b.binding, name: b.Name, kind: b.Kind,
		err: b.Err, typeBound: b.typeBound, deferred: b.deferred,
	}
	t.declared[b.Symbol] = true
	for _, bound := range b.bindings {
		t.declared[bound] = true
	}
	if b.deferred != nil {
		t.img.behaviors = append(t.img.behaviors, img)
		return nil
	}
	var err error
	switch {
	case b.State != nil:
		img.onClock = slices.Contains(t.ctx.clock.waiters, clockWaiter(b.State))
		img.state, err = t.stateExecutor(b.State)
	case b.Action != nil:
		img.onClock = slices.Contains(t.ctx.clock.waiters, clockWaiter(b.Action))
		img.action, err = t.actionExecutor(b.Action)
	default:
		return fmt.Errorf("%w: a behavior with no execution", ErrImageBound)
	}
	if err != nil {
		return err
	}
	t.img.behaviors = append(t.img.behaviors, img)
	return nil
}

// actionExecutor takes an action executor's state, its frames by position.
func (t *imaging) actionExecutor(e *ActionExecutor) (*imagedAction, error) {
	if e.inRun {
		return nil, fmt.Errorf("%w: action %s is running", ErrSnapshotMidRun, symbolText(e.action))
	}
	frames := e.reachableFrames()
	at := func(perf *actionFrame) int { return slices.Index(frames, perf) }
	img := &imagedAction{
		graph: e.graph, state: e.state, nextTokenID: e.nextTokenID,
		nextRepetitionID: e.nextRepetitionID, stepCount: e.stepCount,
		sweep: e.sweep, sweeps: e.sweeps, pausedAt: e.pausedAt, released: e.released,
		pauses: e.pauses, steps: e.steps, stepsSpent: e.stepsSpent, inRun: e.inRun, moved: e.moved,
		awaiting:         at(e.awaiting),
		breakpoints:      maps.Clone(e.breakpoints),
		breakpointNodes:  cloneBreakpoints(e.breakpointNodes),
		firedBreakpoints: maps.Clone(e.firedBreakpoints),
		traversals:       cloneTraversals(e.traversals),
		traversalBase:    e.traversalBase,
		keepTraversals:   e.keepTraversals,
	}
	var err error
	if img.run, err = t.run(e.driven.state); err != nil {
		return nil, err
	}
	if err := t.values(e.inputs); err != nil {
		return nil, fmt.Errorf("inputs: %w", err)
	}
	img.inputs = maps.Clone(e.inputs)
	if e.dynamics != nil {
		if err := t.value(e.dynamics.stepValue); err != nil {
			return nil, fmt.Errorf("time step: %w", err)
		}
		img.dynamics = e.dynamics.clone()
	}
	for _, token := range e.tokens {
		if token.body != nil {
			return nil, fmt.Errorf("%w: token %d of %s at %s", ErrSnapshotPausedBody,
				token.ID, symbolText(e.action), ActionNodeName(token.Location))
		}
		copied := token
		if token.Wait != nil {
			wait := *token.Wait
			copied.Wait = &wait
		}
		copied.frame = nil
		img.tokens = append(img.tokens, copied)
		img.tokenFrames = append(img.tokenFrames, at(token.frame))
	}
	for _, perf := range frames {
		frame, err := t.frame(perf, at)
		if err != nil {
			return nil, err
		}
		img.frames = append(img.frames, frame)
	}
	return img, nil
}

// frame takes one performance's state, the frames it points at by position.
func (t *imaging) frame(perf *actionFrame, at func(*actionFrame) int) (imagedFrame, error) {
	if perf.inBody {
		return imagedFrame{}, fmt.Errorf("%w: a body of %s is running", ErrSnapshotMidRun, perf.label)
	}
	f := imagedFrame{saved: *perf, parent: at(perf.parent)}
	f.saved.parent, f.saved.locals, f.saved.outer, f.saved.data = nil, nil, nil, nil
	f.saved.cells, f.saved.localCells = nil, nil
	f.saved.subactions, f.saved.repeats, f.saved.pending, f.saved.held, f.saved.staged, f.saved.nested = nil, nil, nil, nil, nil, nil
	f.saved.connections = slices.Clone(perf.connections)
	f.saved.features = maps.Clone(perf.features)
	f.saved.aliases = maps.Clone(perf.aliases)
	f.saved.outputs = slices.Clone(perf.outputs)
	f.saved.nodes = slices.Clone(perf.nodes)
	f.saved.streamed = maps.Clone(perf.streamed)
	f.saved.unreceived = cloneUnreceived(perf.unreceived)
	for i, local := range perf.locals {
		cloned := maps.Clone(local)
		var cells []imagedBodyCell
		if i < len(perf.localCells) {
			cells = imageBodyCells(perf.localCells[i], cloned)
		}
		if err := t.values(cloned); err != nil {
			return imagedFrame{}, err
		}
		f.locals = append(f.locals, cloned)
		f.localCells = append(f.localCells, cells)
	}
	f.data = maps.Clone(perf.data)
	f.cells = imageBodyCells(perf.cells, f.data)
	if err := t.values(f.data); err != nil {
		return imagedFrame{}, err
	}
	outer, err := t.outerFrames(perf, at)
	if err != nil {
		return imagedFrame{}, err
	}
	f.outer = outer
	if perf.subactions != nil {
		f.subactions = make(map[ast.Node]int, len(perf.subactions))
		for node, sub := range perf.subactions {
			f.subactions[node] = at(sub)
		}
	}
	if perf.repeats != nil {
		f.repeats = make(map[repetitionGroupID]imagedRepetition, len(perf.repeats))
		for group, state := range perf.repeats {
			repeated := imagedRepetition{node: state.node, remaining: state.remaining, live: make([]int, 0, len(state.live))}
			for _, live := range state.live {
				repeated.live = append(repeated.live, at(live))
			}
			f.repeats[group] = repeated
		}
	}
	if err := t.nestedValues(perf.pending); err != nil {
		return imagedFrame{}, err
	}
	f.pending = clonePending(perf.pending)
	if err := t.heldValues(perf.held); err != nil {
		return imagedFrame{}, err
	}
	f.held = cloneHeld(perf.held)
	f.staged = stagedImaged(perf.staged, at)
	if err := t.deliveredValues(perf.nested); err != nil {
		return imagedFrame{}, err
	}
	f.nested = cloneNested(perf.nested)
	return f, nil
}

// outerFrames images the bindings around perf.
func (t *imaging) outerFrames(perf *actionFrame, at func(*actionFrame) int) ([]imagedOuter, error) {
	var outer []imagedOuter
	for _, o := range perf.outer {
		if o.slots != nil || o.owner != nil {
			return nil, fmt.Errorf("%w: a frame of a calc around %s", ErrImageBound, perf.label)
		}
		if o.perf != nil && at(o.perf) < 0 {
			return nil, fmt.Errorf("%w: a performance around %s the run no longer reaches", ErrImageBound, perf.label)
		}
		if err := t.values(o.vars); err != nil {
			return nil, err
		}
		outer = append(outer, imagedOuter{
			vars: maps.Clone(o.vars), aliases: maps.Clone(o.aliases), perf: at(o.perf),
			performed: o.performed, run: o.run, merged: slices.Clone(o.merged),
		})
	}
	return outer, nil
}

// nestedValues checks every value a pending table holds.
func (t *imaging) nestedValues(pending map[ast.Node]map[string][]Value) error {
	for _, pins := range pending {
		for _, values := range pins {
			for _, v := range values {
				if err := t.value(v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// deliveredValues checks every value a nested-delivery table holds.
func (t *imaging) deliveredValues(nested map[ast.Node][]nestedDelivery) error {
	for _, deliveries := range nested {
		for _, d := range deliveries {
			if err := t.value(d.value); err != nil {
				return err
			}
		}
	}
	return nil
}

// heldValues checks every value a control node holds.
func (t *imaging) heldValues(held map[ast.Node]map[string][]nodeObject) error {
	for _, pins := range held {
		for _, objects := range pins {
			for _, h := range objects {
				if err := t.value(h.value); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// stagedImaged images staged's sources by position.
func stagedImaged(staged map[ast.Node]map[string][]stagedStream, at func(*actionFrame) int) map[ast.Node]map[string][]imagedStaged {
	if staged == nil {
		return nil
	}
	imaged := make(map[ast.Node]map[string][]imagedStaged, len(staged))
	for node, pins := range staged {
		imaged[node] = make(map[string][]imagedStaged, len(pins))
		for pin, entries := range pins {
			for _, s := range entries {
				imaged[node][pin] = append(imaged[node][pin], imagedStaged{source: at(s.source), pin: s.pin, flow: s.flow, at: s.at})
			}
		}
	}
	return imaged
}

// stateExecutor takes a state executor's state.
func (t *imaging) stateExecutor(e *StateExecutor) (*imagedState, error) {
	if e.inRun {
		return nil, fmt.Errorf("%w: state machine %s is running", ErrSnapshotMidRun, symbolText(e.stateMachine))
	}
	img := &imagedState{
		graph: e.graph, state: e.state, activeConfig: cloneConfiguration(e.activeConfig),
		nextEventID:        e.nextEventID,
		stateData:          maps.Clone(e.stateData),
		stateAttrCells:     make(map[*ast.StateNode][]imagedBodyCell, len(e.stateAttrCells)),
		stateAttrs:         make(map[*ast.StateNode]map[string]Value, len(e.stateAttrs)),
		stateVisits:        slices.Clone(e.stateVisits),
		stateStack:         slices.Clone(e.stateStack),
		fired:              slices.Clone(e.fired),
		firedBase:          e.firedBase,
		keepFired:          e.keepFired,
		breakpointNodes:    maps.Clone(e.breakpointNodes),
		breakpointHit:      e.breakpointHit,
		pausedAt:           e.pausedAt,
		completionDue:      e.completionDue,
		history:            make(map[*ast.StateNode]historyRecord, len(e.history)),
		joinArrived:        cloneJoinArrivals(e.joinArrived),
		lastDispatch:       cloneDispatch(e.lastDispatch),
		lastEventAt:        e.lastEventAt,
		machineExited:      e.machineExited,
		inRun:              e.inRun,
		moved:              e.moved,
		timerScheduled:     maps.Clone(e.timerScheduled),
		timeTriggerVerdict: maps.Clone(e.timeTriggerVerdict),
		changeFired:        maps.Clone(e.changeFired),
		changeObserved:     maps.Clone(e.changeObserved),
		changePending:      maps.Clone(e.changePending),
		firingChange:       e.firingChange,
		firingNotes:        slices.Clone(e.firingNotes),
		changeRearmed:      maps.Clone(e.changeRearmed),
		changeWaits:        slices.Clone(e.changeWaits),
		held:               cloneHeldEntries(e.held),
		entering:           maps.Clone(e.entering),
		enteringMachine:    e.enteringMachine,
		activeAtEntry:      maps.Clone(e.activeAtEntry),
		pendingCall:        e.pendingCall.clone(),
	}
	var err error
	if img.run, err = t.run(e.driven.state); err != nil {
		return nil, err
	}
	img.stateCells = imageBodyCells(e.stateCells, img.stateData)
	if err := t.values(img.stateData); err != nil {
		return nil, err
	}
	if call := e.pendingCall; call != nil {
		if err := t.values(call.outputs); err != nil {
			return nil, fmt.Errorf("call: %w", err)
		}
		if err := t.values(call.inouts); err != nil {
			return nil, fmt.Errorf("call: %w", err)
		}
	}
	for node, attrs := range e.stateAttrs {
		cloned := maps.Clone(attrs)
		img.stateAttrCells[node] = imageBodyCells(e.stateAttrCells[node], cloned)
		if err := t.values(cloned); err != nil {
			return nil, fmt.Errorf("state %s: %w", StateVertexName(node), err)
		}
		img.stateAttrs[node] = cloned
	}
	for node, record := range e.history {
		img.history[node] = historyRecord{child: record.child, regions: maps.Clone(record.regions)}
	}
	if e.eventQueue != nil {
		img.events = slices.Clone(e.eventQueue.events)
	}
	for _, event := range img.events {
		if err := t.event(event); err != nil {
			return nil, err
		}
	}
	if img.lastDispatch != nil {
		if err := t.event(img.lastDispatch.Event); err != nil {
			return nil, err
		}
	}
	for _, act := range e.doActions {
		if act.run != nil {
			return nil, fmt.Errorf("%w: do behavior of state %s of %s", ErrSnapshotPausedBody,
				StateVertexName(act.state), symbolText(e.stateMachine))
		}
		if err := t.firing(act.firing); err != nil {
			return nil, fmt.Errorf("do behavior of state %s: %w", StateVertexName(act.state), err)
		}
		img.doActions = append(img.doActions, doActionCapture{act: &doAction{state: act.state}, pending: slices.Clone(act.pending), firing: act.firing.snapshot()})
	}
	for _, item := range img.held {
		if err := t.firing(item.firing); err != nil {
			return nil, fmt.Errorf("held entry of %s: %w", StateVertexName(item.owner), err)
		}
	}
	return img, nil
}

// firing checks that the payload a firing bound carries.
func (t *imaging) firing(f *firing) error {
	if f == nil {
		return nil
	}
	return t.values(f.payload)
}

// event checks that an event's payload carries, reaching what it names.
func (t *imaging) event(event Event) error {
	switch payload := event.Payload.(type) {
	case nil, *lower.Transition, *ast.TransitionEdge:
		return nil
	case Message:
		return t.message(payload)
	case Call:
		if err := t.values(payload.Args); err != nil {
			return fmt.Errorf("call %s: %w", payload.Operation, err)
		}
		return nil
	}
	return fmt.Errorf("%w: event %d carries a %T", ErrImageBound, event.ID, event.Payload)
}

// behavior gives the object made for an imaged behavior an execution of its own
// standing where the imaged one stood.
func (m *materializing) behavior(b imagedBehavior) error {
	dst := m.dst
	inst := m.made[b.object]
	decl, ok := m.declaration(inst, b.member)
	if !ok {
		return fmt.Errorf("%w: the type binds no such behavior", ErrImageBound)
	}
	if b.deferred != nil {
		behavior, err := dst.deferredBehaviorFor(inst, decl, b.binding)
		if err != nil {
			return err
		}
		behavior.Err = b.err
		behavior.typeBound = b.typeBound
		behavior.Name = b.name
		behavior.Kind = b.kind
		inst.behaviors = append(inst.behaviors, behavior)
		dst.behaviorsAttached++
		dst.objectBehaviors = append(dst.objectBehaviors, behavior)
		dst.workChanged()
		return nil
	}
	behavior, occurrence, err := dst.bindClassifierBehavior(inst, decl)
	if err != nil {
		return err
	}
	behavior.binding = b.binding
	behavior.Err = b.err
	behavior.typeBound = b.typeBound
	switch {
	case b.state != nil:
		exec := newStateExecutorOn(dst, behavior.Symbol, inst, occurrence, b.state.graph)
		if err := m.stateExecutor(exec, b.state); err != nil {
			return err
		}
		dst.registerStateExecutor(exec)
		if b.onClock {
			dst.clock.attach(exec)
		}
		behavior.State = exec
	case b.action != nil:
		action, tool, _, err := dst.performanceBody(decl.member, behavior.Symbol)
		if err != nil {
			return err
		}
		exec := newActionExecutorOn(dst, decl.member, action, tool, b.action.graph, inst, occurrence)
		if err := m.actionExecutor(exec, b.action); err != nil {
			dst.clock.detach(exec)
			return err
		}
		if !b.onClock {
			dst.clock.detach(exec)
		}
		behavior.Action = exec
	}
	inst.behaviors = append(inst.behaviors, behavior)
	dst.behaviorsAttached++
	dst.objectBehaviors = append(dst.objectBehaviors, behavior)
	dst.workChanged()
	return nil
}

// declaration finds, among the behaviors the object's types bind or declare for a
// start, the one member declares.
func (m *materializing) declaration(inst *Instance, member *symbols.Symbol) (classifierBehaviorDecl, bool) {
	for _, typ := range inst.types() {
		for _, decl := range m.dst.classifierBehaviorsOf(typ) {
			if decl.member == member {
				return decl, true
			}
		}
	}
	return m.dst.startableDeclaration(inst, member)
}

// runOf is the run of dst's own made for an imaged run, nil for none.
func (m *materializing) runOf(at int) *runState {
	if at < 0 {
		return nil
	}
	return m.runs[at]
}

// actionExecutor puts an imaged action's state on a fresh execution of dst's.
func (m *materializing) actionExecutor(e *ActionExecutor, img *imagedAction) error {
	frames := make([]*actionFrame, len(img.frames))
	for i := range frames {
		frames[i] = &actionFrame{perfs: &e.performances}
	}
	frameAt := func(at int) *actionFrame {
		if at < 0 {
			return nil
		}
		return frames[at]
	}
	for i, f := range img.frames {
		if err := m.frame(frames[i], f, frameAt); err != nil {
			return err
		}
	}
	if len(frames) > 0 {
		e.root = frames[0]
	}
	e.tokens = make([]Token, 0, len(img.tokens))
	for i, token := range img.tokens {
		copied := token
		if token.Wait != nil {
			wait := *token.Wait
			copied.Wait = &wait
		}
		copied.frame = frameAt(img.tokenFrames[i])
		e.tokens = append(e.tokens, copied)
	}
	e.state, e.nextTokenID, e.nextRepetitionID, e.stepCount, e.sweep, e.sweeps =
		img.state, img.nextTokenID, img.nextRepetitionID, img.stepCount, img.sweep, img.sweeps
	e.pausedAt, e.released, e.pauses = img.pausedAt, img.released, img.pauses
	e.steps, e.stepsSpent, e.inRun, e.moved = img.steps, img.stepsSpent, img.inRun, img.moved
	e.awaiting = frameAt(img.awaiting)
	e.breakpoints = maps.Clone(img.breakpoints)
	if e.breakpoints == nil {
		e.breakpoints = make(map[string]bool)
	}
	e.breakpointNodes = cloneBreakpoints(img.breakpointNodes)
	e.traversals, e.traversalBase, e.keepTraversals = cloneTraversals(img.traversals), img.traversalBase, img.keepTraversals
	e.firedBreakpoints = maps.Clone(img.firedBreakpoints)
	if e.firedBreakpoints == nil {
		e.firedBreakpoints = make(map[breakpointVisit]bool)
	}
	var err error
	if e.inputs, err = m.values(img.inputs); err != nil {
		return fmt.Errorf("inputs: %w", err)
	}
	e.driven.state = m.runOf(img.run)
	if img.dynamics != nil {
		e.dynamics = img.dynamics.clone()
		if e.dynamics.stepValue, err = m.value(img.dynamics.stepValue); err != nil {
			return fmt.Errorf("time step: %w", err)
		}
	}
	return nil
}

// frame fills one performance of dst's from its image.
func (m *materializing) frame(perf *actionFrame, img imagedFrame, frameAt func(int) *actionFrame) error {
	perfs := perf.perfs
	*perf = img.saved
	perf.perfs = perfs
	perf.parent = frameAt(img.parent)
	perf.connections = slices.Clone(img.saved.connections)
	perf.features = maps.Clone(img.saved.features)
	perf.aliases = maps.Clone(img.saved.aliases)
	perf.outputs = slices.Clone(img.saved.outputs)
	perf.nodes = slices.Clone(img.saved.nodes)
	perf.streamed = maps.Clone(img.saved.streamed)
	perf.unreceived = cloneUnreceived(img.saved.unreceived)
	var err error
	perf.locals = nil
	perf.localCells = make([]*bodyCells, len(img.locals))
	for i, local := range img.locals {
		carried, err := m.values(local)
		if err != nil {
			return err
		}
		perf.locals = append(perf.locals, carried)
		if i < len(img.localCells) {
			perf.localCells[i] = m.frameCells(perf, carried, img.localCells[i], i+1)
		}
	}
	if perf.data, err = m.values(img.data); err != nil {
		return err
	}
	perf.cells = m.frameCells(perf, perf.data, img.cells, 0)
	perf.outer = nil
	for _, outer := range img.outer {
		vars, err := m.values(outer.vars)
		if err != nil {
			return err
		}
		perf.outer = append(perf.outer, frame{
			vars: vars, aliases: maps.Clone(outer.aliases), perf: frameAt(outer.perf),
			performed: outer.performed, run: outer.run, merged: slices.Clone(outer.merged),
		})
	}
	if img.subactions != nil {
		perf.subactions = make(map[ast.Node]*actionFrame, len(img.subactions))
		for node, at := range img.subactions {
			perf.subactions[node] = frameAt(at)
		}
	}
	if img.repeats != nil {
		perf.repeats = make(map[repetitionGroupID]*stepRepetition, len(img.repeats))
		for group, repeated := range img.repeats {
			state := &stepRepetition{node: repeated.node, remaining: repeated.remaining, live: make([]*actionFrame, 0, len(repeated.live))}
			for _, at := range repeated.live {
				state.live = append(state.live, frameAt(at))
			}
			perf.repeats[group] = state
		}
	}
	if err := m.pending(perf, img.pending); err != nil {
		return err
	}
	if err := m.held(perf, img.held); err != nil {
		return err
	}
	perf.staged = stagedMaterialized(img.staged, frameAt)
	if err := m.nested(perf, img.nested); err != nil {
		return err
	}
	for _, cells := range perf.localCells {
		if err := m.dst.deriveBodyCells(cells); err != nil {
			return err
		}
	}
	if err := m.dst.deriveBodyCells(perf.cells); err != nil {
		return err
	}
	return nil
}

// imageBodyCells captures binding declarations and flags without dependency edges.
func imageBodyCells(cells *bodyCells, values map[string]Value) []imagedBodyCell {
	if cells == nil {
		return nil
	}
	// Tracking cells omit dependency edges from images and derive again on first read.
	var image []imagedBodyCell
	for _, name := range cells.order {
		cell := cells.cells[name]
		if cell.binding == nil {
			continue
		}
		image = append(image, imagedBodyCell{
			name: name, value: cell.binding.value, scope: cell.binding.scope,
			masked:  cell.binding.masked,
			written: cell.fv.Written, frozen: cell.binding.frozen,
			visible: cloneBodyBindingVisibility(cell.binding.visible), limitFrames: cell.binding.limitFrames,
		})
		if !cell.fv.Written && !cell.binding.frozen {
			delete(values, name)
		}
	}
	return image
}

// restoreStateBodyCells reinstates the binding cells owned by a state image.
func restoreStateBodyCells(
	ctx *Context,
	cells *bodyCells,
	attributes []lower.Attribute,
	image []imagedBodyCell,
	owner string,
	defaultScope *symbols.Scope,
) {
	features := make(map[string]lower.Attribute, len(attributes))
	for _, attr := range attributes {
		features[attr.Name] = attr
	}
	for _, state := range image {
		attr, ok := features[state.name]
		if !ok || !attr.Binding || attr.Value == nil {
			continue
		}
		if cell := cells.cells[state.name]; cell != nil && cell.binding != nil {
			restoreBodyCellValue(cells, state, cell)
			continue
		}
		scope := attr.Scope
		if scope == nil {
			scope = defaultScope
		}
		check := func(value *Value) error {
			return ctx.checkBodyDeclaration(scope, owner, state.name, value)
		}
		cell := ctx.registerBodyBinding(cells, state.name, attr.Value, scope, check, nil)
		cell.binding.visible = cloneBodyBindingVisibility(state.visible)
		cell.binding.limitFrames = state.limitFrames
		restoreBodyCellValue(cells, state, cell)
	}
}

// restoreBodyCellValue restores a body's value and written or tracking state.
func restoreBodyCellValue(cells *bodyCells, state imagedBodyCell, cell *bodyCell) {
	if value, held := cells.vars[state.name]; held {
		cell.fv.Value, cell.fv.Materialized = value, true
	}
	if state.written {
		cell.fv.Written = true
	} else if state.frozen {
		cell.binding.frozen = true
	} else {
		delete(cells.vars, state.name)
		cell.fv.Value, cell.fv.Values, cell.fv.Materialized = Value{}, Value{}, false
	}
}

// frameCells rebuilds the dependency cells for a materialized performance frame.
func (m *materializing) frameCells(
	perf *actionFrame,
	values map[string]Value,
	image []imagedBodyCell,
	localDepth int,
) *bodyCells {
	if len(image) == 0 {
		return nil
	}
	defaultContext := func(scope *symbols.Scope) *EvalContext {
		ec := perf.perfs.evalContextFor(perf, scope)
		if localDepth > 0 {
			localStart := len(ec.frames) - len(perf.locals) - 1
			ec.frames = ec.frames[:localStart+localDepth]
		}
		return ec
	}
	cells := newBodyCells(values, defaultContext)
	for _, state := range image {
		if state.value == nil {
			continue
		}
		check := func(value *Value) error {
			return m.dst.checkBodyDeclarationAs(state.scope, perf.describe, state.name, value)
		}
		var onDerived func(*Value) error
		if perf.node == nil && perf.parent == nil {
			if owner, ok := perf.perfs.owner.(*ActionExecutor); ok {
				cell := cells.cell(state.name)
				onDerived = func(value *Value) error {
					mirrored, err := owner.mirrorBindingOccurrence(state.name, *value, cell)
					if err != nil {
						return err
					}
					*value = mirrored
					return nil
				}
			}
		}
		context := func(scope *symbols.Scope) *EvalContext {
			if state.masked != "" {
				return perf.perfs.evalBindingContext(perf, scope, state.masked, perf.began)
			}
			return defaultContext(scope)
		}
		cell := m.dst.registerBodyBindingInContext(cells, state.name, state.value, state.scope, check, context, onDerived)
		cell.binding.visible = cloneBodyBindingVisibility(state.visible)
		cell.binding.masked = state.masked
		cell.binding.limitFrames = state.limitFrames
		if state.written {
			cell.fv.Written = true
		} else if state.frozen {
			cell.binding.frozen = true
		} else {
			delete(values, state.name)
			cell.fv.Value, cell.fv.Materialized = Value{}, false
		}
	}
	return cells
}

// pending fills perf's pending table from img's.
func (m *materializing) pending(perf *actionFrame, pending map[ast.Node]map[string][]Value) error {
	if pending == nil {
		return nil
	}
	perf.pending = make(map[ast.Node]map[string][]Value, len(pending))
	for node, pins := range pending {
		carriedPins := make(map[string][]Value, len(pins))
		for pin, values := range pins {
			for _, v := range values {
				carried, err := m.value(v)
				if err != nil {
					return err
				}
				carriedPins[pin] = append(carriedPins[pin], carried)
			}
		}
		perf.pending[node] = carriedPins
	}
	return nil
}

// held fills what perf's control nodes hold from img's.
func (m *materializing) held(perf *actionFrame, held map[ast.Node]map[string][]nodeObject) error {
	if held == nil {
		return nil
	}
	perf.held = make(map[ast.Node]map[string][]nodeObject, len(held))
	for node, pins := range held {
		carriedPins := make(map[string][]nodeObject, len(pins))
		for pin, objects := range pins {
			for _, h := range objects {
				carried, err := m.value(h.value)
				if err != nil {
					return err
				}
				h.value = carried
				carriedPins[pin] = append(carriedPins[pin], h)
			}
		}
		perf.held[node] = carriedPins
	}
	return nil
}

// nested fills perf's nested-delivery table from img's.
func (m *materializing) nested(perf *actionFrame, nested map[ast.Node][]nestedDelivery) error {
	if nested == nil {
		return nil
	}
	perf.nested = make(map[ast.Node][]nestedDelivery, len(nested))
	for node, deliveries := range nested {
		for _, d := range deliveries {
			carried, err := m.value(d.value)
			if err != nil {
				return err
			}
			perf.nested[node] = append(perf.nested[node], nestedDelivery{path: slices.Clone(d.path), pin: d.pin, value: carried})
		}
	}
	return nil
}

// stagedMaterialized materializes img's staged writes, their sources by position.
func stagedMaterialized(staged map[ast.Node]map[string][]imagedStaged, frameAt func(int) *actionFrame) map[ast.Node]map[string][]stagedStream {
	if staged == nil {
		return nil
	}
	materialized := make(map[ast.Node]map[string][]stagedStream, len(staged))
	for node, pins := range staged {
		materialized[node] = make(map[string][]stagedStream, len(pins))
		for pin, entries := range pins {
			for _, s := range entries {
				materialized[node][pin] = append(materialized[node][pin], stagedStream{source: frameAt(s.source), pin: s.pin, flow: s.flow, at: s.at})
			}
		}
	}
	return materialized
}

// stateExecutor puts an imaged machine's state on a fresh execution of dst's.
func (m *materializing) stateExecutor(e *StateExecutor, img *imagedState) error {
	var err error
	e.state, e.activeConfig, e.nextEventID = img.state, cloneConfiguration(img.activeConfig), img.nextEventID
	if e.stateData, err = m.values(img.stateData); err != nil {
		return err
	}
	if e.stateData == nil {
		e.stateData = make(map[string]Value)
	}
	if e.stateCells != nil {
		e.stateCells.vars = e.stateData
		restoreStateBodyCells(m.dst, e.stateCells, e.graph.Attributes, img.stateCells,
			"state machine "+symbolText(e.stateMachine), e.graph.Scope)
	}
	for node, attrs := range img.stateAttrs {
		if e.stateAttrs[node], err = m.values(attrs); err != nil {
			return fmt.Errorf("state %s: %w", StateVertexName(node), err)
		}
		if image := img.stateAttrCells[node]; len(image) != 0 {
			cells := newBodyCells(e.stateAttrs[node], func(scope *symbols.Scope) *EvalContext {
				return e.stateAttributeContext(node, scope)
			})
			restoreStateBodyCells(m.dst, cells, e.graph.StateAttributes[node], image,
				"state "+node.Name, e.graph.Scope)
			e.stateAttrCells[node] = cells
		}
	}
	e.stateVisits, e.stateStack = slices.Clone(img.stateVisits), slices.Clone(img.stateStack)
	e.fired, e.firedBase, e.keepFired = slices.Clone(img.fired), img.firedBase, img.keepFired
	e.breakpointNodes = maps.Clone(img.breakpointNodes)
	if e.breakpointNodes == nil {
		e.breakpointNodes = make(map[ast.Node]bool)
	}
	e.breakpointHit, e.pausedAt, e.completionDue = img.breakpointHit, img.pausedAt, img.completionDue
	e.joinArrived = cloneJoinArrivals(img.joinArrived)
	for node, record := range img.history {
		e.history[node] = &historyRecord{child: record.child, regions: maps.Clone(record.regions)}
	}
	e.eventQueue.events = make(eventHeap, 0, len(img.events))
	for _, event := range img.events {
		carried, err := m.event(event)
		if err != nil {
			return err
		}
		e.eventQueue.events = append(e.eventQueue.events, carried)
	}
	if img.lastDispatch != nil {
		dispatch := cloneDispatch(img.lastDispatch)
		if dispatch.Event, err = m.event(img.lastDispatch.Event); err != nil {
			return err
		}
		e.lastDispatch = dispatch
	}
	e.lastEventAt = img.lastEventAt
	for _, act := range img.doActions {
		carried, err := m.firing(act.firing)
		if err != nil {
			return fmt.Errorf("do behavior of state %s: %w", StateVertexName(act.act.state), err)
		}
		e.doActions = append(e.doActions, &doAction{state: act.act.state, pending: slices.Clone(act.pending), firing: carried})
	}
	e.machineExited, e.inRun, e.moved = img.machineExited, img.inRun, img.moved
	e.driven.state = m.runOf(img.run)
	e.timerScheduled = maps.Clone(img.timerScheduled)
	e.timeTriggerVerdict = maps.Clone(img.timeTriggerVerdict)
	e.changeFired = maps.Clone(img.changeFired)
	e.changeObserved = maps.Clone(img.changeObserved)
	e.changePending = maps.Clone(img.changePending)
	e.changeReads = make(map[*lower.Transition][]*FeatureValue)
	e.firingChange, e.firingNotes = img.firingChange, slices.Clone(img.firingNotes)
	e.changeRearmed = maps.Clone(img.changeRearmed)
	e.changeWaits = slices.Clone(img.changeWaits)
	e.held = cloneHeldEntries(img.held)
	for i := range e.held {
		if e.held[i].firing, err = m.firing(img.held[i].firing); err != nil {
			return fmt.Errorf("held entry of %s: %w", StateVertexName(e.held[i].owner), err)
		}
	}
	clear(e.entering)
	for state, entering := range img.entering {
		e.entering[state] = entering
	}
	e.enteringMachine = img.enteringMachine
	e.activeAtEntry = maps.Clone(img.activeAtEntry)
	return m.pendingCall(e, img.pendingCall)
}

// pendingCall gives e the imaged call, its values carried as dst's own.
func (m *materializing) pendingCall(e *StateExecutor, call *pendingCall) error {
	if call == nil {
		e.pendingCall = nil
		return nil
	}
	carried := call.clone()
	var err error
	if carried.outputs, err = m.values(call.outputs); err != nil {
		return err
	}
	if carried.inouts, err = m.values(call.inouts); err != nil {
		return err
	}
	e.pendingCall = carried
	return nil
}

// firing is a firing as dst carries it, the payload it bound as dst's own values.
func (m *materializing) firing(f *firing) (*firing, error) {
	if f == nil {
		return nil, nil
	}
	payload, err := m.values(f.payload)
	if err != nil {
		return nil, err
	}
	return &firing{taken: f.taken, payload: payload}, nil
}

// event is an event as dst carries it.
func (m *materializing) event(event Event) (Event, error) {
	out := event
	switch payload := event.Payload.(type) {
	case Message:
		carried, err := m.message(payload)
		if err != nil {
			return Event{}, err
		}
		out.Payload = carried
	case Call:
		args, err := m.values(payload.Args)
		if err != nil {
			return Event{}, fmt.Errorf("call %s: %w", payload.Operation, err)
		}
		out.Payload = Call{Operation: payload.Operation, Declared: payload.Declared, Args: args}
	}
	return out, nil
}
