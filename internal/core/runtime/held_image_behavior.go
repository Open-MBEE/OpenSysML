package runtime

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// imagedBehavior is one object's execution of a behavior its type binds, by value.
// The lowered graph is kept as is: lowered IR is derived from the shared, frozen
// declarations and never written once lowered, so every context reads one copy.
type imagedBehavior struct {
	object   int64
	attached int // position among the behaviors of the context imaged
	member   *symbols.Symbol
	binding  int
	name     string
	kind     lower.ClassifierBehaviorKind
	onClock  bool
	action   *imagedAction
	state    *imagedState
}

// imagedAction is an action executor's state by value, its frames by position.
type imagedAction struct {
	graph             *lower.ActionGraph
	tokens            []Token
	tokenFrames       []int
	state             ExecutionState
	nextTokenID       int64
	stepCount         int
	sweep, sweeps     uint64
	inputs            map[string]Value
	pausedAt          string
	released          bool
	pauses            int64
	steps, stepsSpent int64
	inRun, moved      bool
	awaiting          int
	breakpoints       map[string]bool
	firedBreakpoints  map[breakpointVisit]bool
	run               int
	frames            []imagedFrame
}

// imagedFrame is one performance's state by value, the frames it points at by position.
type imagedFrame struct {
	saved      actionFrame
	parent     int
	locals     []map[string]Value
	data       map[string]Value
	outer      []imagedOuter
	subactions map[ast.Node]int
	pending    map[ast.Node]map[string][]Value
	nested     map[ast.Node][]nestedDelivery
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
	stateAttrs         map[*ast.StateNode]map[string]Value
	stateVisits        []string
	stateStack         []*ast.StateNode
	history            map[*ast.StateNode]historyRecord
	deferred           []Event
	lastDispatch       *Dispatch
	lastEventAt        float64
	doActions          []doActionCapture
	machineExited      bool
	run                int
	inRun, moved       bool
	timerScheduled     map[*lower.Transition]bool
	timeTriggerVerdict map[*lower.Transition]error
	changeFired        map[*lower.Transition]bool
	firingChange       *lower.Transition
	firingNotes        []RunNote
	changeRearmed      map[*lower.Transition]bool
	changeWaits        []changeWait
}

// behavior takes one behavior's execution.
func (t *imaging) behavior(b *ObjectBehavior) error {
	img := imagedBehavior{
		object: b.Object.ID, attached: slices.Index(t.ctx.objectBehaviors, b),
		member: b.member, binding: b.binding, name: b.Name, kind: b.Kind,
	}
	t.declared[b.Symbol] = true
	for _, bound := range b.bindings {
		t.declared[bound] = true
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
		graph: e.graph, state: e.state, nextTokenID: e.nextTokenID, stepCount: e.stepCount,
		sweep: e.sweep, sweeps: e.sweeps, pausedAt: e.pausedAt, released: e.released,
		pauses: e.pauses, steps: e.steps, stepsSpent: e.stepsSpent, inRun: e.inRun, moved: e.moved,
		awaiting:         at(e.awaiting),
		breakpoints:      maps.Clone(e.breakpoints),
		firedBreakpoints: maps.Clone(e.firedBreakpoints),
	}
	var err error
	if img.run, err = t.run(e.driven.state); err != nil {
		return nil, err
	}
	if err := t.values(e.inputs); err != nil {
		return nil, fmt.Errorf("inputs: %w", err)
	}
	img.inputs = maps.Clone(e.inputs)
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
	f.saved.subactions, f.saved.pending, f.saved.nested = nil, nil, nil
	f.saved.connections = slices.Clone(perf.connections)
	f.saved.features = maps.Clone(perf.features)
	f.saved.aliases = maps.Clone(perf.aliases)
	f.saved.outputs = slices.Clone(perf.outputs)
	f.saved.nodes = slices.Clone(perf.nodes)
	for _, local := range perf.locals {
		if err := t.values(local); err != nil {
			return imagedFrame{}, err
		}
		f.locals = append(f.locals, maps.Clone(local))
	}
	if err := t.values(perf.data); err != nil {
		return imagedFrame{}, err
	}
	f.data = maps.Clone(perf.data)
	for _, outer := range perf.outer {
		if outer.slots != nil || outer.owner != nil {
			return imagedFrame{}, fmt.Errorf("%w: a frame of a calc around %s", ErrImageBound, perf.label)
		}
		if outer.perf != nil && at(outer.perf) < 0 {
			return imagedFrame{}, fmt.Errorf("%w: a performance around %s the run no longer reaches", ErrImageBound, perf.label)
		}
		if err := t.values(outer.vars); err != nil {
			return imagedFrame{}, err
		}
		f.outer = append(f.outer, imagedOuter{
			vars: maps.Clone(outer.vars), aliases: maps.Clone(outer.aliases), perf: at(outer.perf),
			performed: outer.performed, run: outer.run, merged: slices.Clone(outer.merged),
		})
	}
	if perf.subactions != nil {
		f.subactions = make(map[ast.Node]int, len(perf.subactions))
		for node, sub := range perf.subactions {
			f.subactions[node] = at(sub)
		}
	}
	for _, pins := range perf.pending {
		for _, values := range pins {
			for _, v := range values {
				if err := t.value(v); err != nil {
					return imagedFrame{}, err
				}
			}
		}
	}
	f.pending = clonePending(perf.pending)
	for _, deliveries := range perf.nested {
		for _, d := range deliveries {
			if err := t.value(d.value); err != nil {
				return imagedFrame{}, err
			}
		}
	}
	f.nested = cloneNested(perf.nested)
	return f, nil
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
		stateAttrs:         make(map[*ast.StateNode]map[string]Value, len(e.stateAttrs)),
		stateVisits:        slices.Clone(e.stateVisits),
		stateStack:         slices.Clone(e.stateStack),
		history:            make(map[*ast.StateNode]historyRecord, len(e.history)),
		deferred:           slices.Clone(e.deferred),
		lastDispatch:       cloneDispatch(e.lastDispatch),
		lastEventAt:        e.lastEventAt,
		machineExited:      e.machineExited,
		inRun:              e.inRun,
		moved:              e.moved,
		timerScheduled:     maps.Clone(e.timerScheduled),
		timeTriggerVerdict: maps.Clone(e.timeTriggerVerdict),
		changeFired:        maps.Clone(e.changeFired),
		firingChange:       e.firingChange,
		firingNotes:        slices.Clone(e.firingNotes),
		changeRearmed:      maps.Clone(e.changeRearmed),
		changeWaits:        slices.Clone(e.changeWaits),
	}
	var err error
	if img.run, err = t.run(e.driven.state); err != nil {
		return nil, err
	}
	if err := t.values(e.stateData); err != nil {
		return nil, err
	}
	for node, attrs := range e.stateAttrs {
		if err := t.values(attrs); err != nil {
			return nil, fmt.Errorf("state %s: %w", getNodeName(node), err)
		}
		img.stateAttrs[node] = maps.Clone(attrs)
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
	for _, event := range img.deferred {
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
				getNodeName(act.state), symbolText(e.stateMachine))
		}
		img.doActions = append(img.doActions, doActionCapture{act: &doAction{state: act.state}, pending: slices.Clone(act.pending)})
	}
	return img, nil
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
	behavior, occurrence, err := dst.bindClassifierBehavior(inst, decl)
	if err != nil {
		return err
	}
	behavior.binding = b.binding
	switch {
	case b.state != nil:
		exec := newStateExecutorOn(dst, behavior.Symbol, inst, occurrence, b.state.graph)
		if err := m.stateExecutor(exec, b.state); err != nil {
			return err
		}
		if b.onClock {
			dst.clock.attach(exec)
		}
		behavior.State = exec
	case b.action != nil:
		action, tool, err := dst.performanceBody(decl.member, behavior.Symbol)
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
	dst.objectBehaviors = append(dst.objectBehaviors, behavior)
	return nil
}

// declaration finds, among the behaviors the object's types bind, the one member declares.
func (m *materializing) declaration(inst *Instance, member *symbols.Symbol) (classifierBehaviorDecl, bool) {
	for _, typ := range inst.types() {
		for _, decl := range m.dst.classifierBehaviorsOf(typ) {
			if decl.member == member {
				return decl, true
			}
		}
	}
	return classifierBehaviorDecl{}, false
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
		frames[i] = &actionFrame{}
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
	e.state, e.nextTokenID, e.stepCount, e.sweep, e.sweeps = img.state, img.nextTokenID, img.stepCount, img.sweep, img.sweeps
	e.pausedAt, e.released, e.pauses = img.pausedAt, img.released, img.pauses
	e.steps, e.stepsSpent, e.inRun, e.moved = img.steps, img.stepsSpent, img.inRun, img.moved
	e.awaiting = frameAt(img.awaiting)
	e.breakpoints = maps.Clone(img.breakpoints)
	if e.breakpoints == nil {
		e.breakpoints = make(map[string]bool)
	}
	e.firedBreakpoints = maps.Clone(img.firedBreakpoints)
	if e.firedBreakpoints == nil {
		e.firedBreakpoints = make(map[breakpointVisit]bool)
	}
	var err error
	if e.inputs, err = m.values(img.inputs); err != nil {
		return fmt.Errorf("inputs: %w", err)
	}
	e.driven.state = m.runOf(img.run)
	return nil
}

// frame fills one performance of dst's from its image.
func (m *materializing) frame(perf *actionFrame, img imagedFrame, frameAt func(int) *actionFrame) error {
	*perf = img.saved
	perf.parent = frameAt(img.parent)
	perf.connections = slices.Clone(img.saved.connections)
	perf.features = maps.Clone(img.saved.features)
	perf.aliases = maps.Clone(img.saved.aliases)
	perf.outputs = slices.Clone(img.saved.outputs)
	perf.nodes = slices.Clone(img.saved.nodes)
	var err error
	perf.locals = nil
	for _, local := range img.locals {
		carried, err := m.values(local)
		if err != nil {
			return err
		}
		perf.locals = append(perf.locals, carried)
	}
	if perf.data, err = m.values(img.data); err != nil {
		return err
	}
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
	if img.pending != nil {
		perf.pending = make(map[ast.Node]map[string][]Value, len(img.pending))
		for node, pins := range img.pending {
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
	}
	if img.nested != nil {
		perf.nested = make(map[ast.Node][]nestedDelivery, len(img.nested))
		for node, deliveries := range img.nested {
			for _, d := range deliveries {
				carried, err := m.value(d.value)
				if err != nil {
					return err
				}
				perf.nested[node] = append(perf.nested[node], nestedDelivery{path: slices.Clone(d.path), pin: d.pin, value: carried})
			}
		}
	}
	return nil
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
	for node, attrs := range img.stateAttrs {
		if e.stateAttrs[node], err = m.values(attrs); err != nil {
			return fmt.Errorf("state %s: %w", getNodeName(node), err)
		}
	}
	e.stateVisits, e.stateStack = slices.Clone(img.stateVisits), slices.Clone(img.stateStack)
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
	e.deferred = make([]Event, 0, len(img.deferred))
	for _, event := range img.deferred {
		carried, err := m.event(event)
		if err != nil {
			return err
		}
		e.deferred = append(e.deferred, carried)
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
		e.doActions = append(e.doActions, &doAction{state: act.act.state, pending: slices.Clone(act.pending)})
	}
	e.machineExited, e.inRun, e.moved = img.machineExited, img.inRun, img.moved
	e.driven.state = m.runOf(img.run)
	e.timerScheduled = maps.Clone(img.timerScheduled)
	e.timeTriggerVerdict = maps.Clone(img.timeTriggerVerdict)
	e.changeFired = maps.Clone(img.changeFired)
	e.firingChange, e.firingNotes = img.firingChange, slices.Clone(img.firingNotes)
	e.changeRearmed = maps.Clone(img.changeRearmed)
	e.changeWaits = slices.Clone(img.changeWaits)
	return nil
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
		out.Payload = Call{Operation: payload.Operation, Args: args}
	}
	return out, nil
}
