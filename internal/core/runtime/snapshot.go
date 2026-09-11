package runtime

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// ErrSnapshotMidRun reports a snapshot asked for from inside a step: a snapshot is
// taken between steps, where no run, body or evaluation is on the stack.
var ErrSnapshotMidRun = errors.New("snapshot inside a step")

// ErrSnapshotPausedBody reports a body paused mid-statement — a token's step a
// breakpoint or a wait on the clock suspended, or a do behavior waiting — whose
// continuation is a coroutine no snapshot captures.
var ErrSnapshotPausedBody = errors.New("snapshot of a body paused mid-statement")

// Snapshot is the run-derived state of a Context at one point between steps: a
// mark in its journal, which Restore brings the run back to as often as asked
// until Release. The Model the context runs over is not part of it. Every object
// the run had made keeps its identity across a restore; what it made after the
// mark is abandoned, and the identities it took are handed out again.
type Snapshot struct {
	ctx       *Context
	journal   journalMark
	run       runCapture
	runStates []runStateCapture
	actions   []actionCapture
	states    []stateCapture
	released  bool
}

// journalMark is where in the journal a change began and what the journal holds
// by value: the bus, the clock, the objects made and behaviors attached so far.
type journalMark struct {
	writes, undos     int
	created, attached int
	messages          []Message
	clockNow          float64
	clockWaiters      []clockWaiter
}

// runCapture is the run bookkeeping the context keeps outside its journal.
type runCapture struct {
	ids                *idSequence
	nextID             int64
	activations, runs  int64
	run                *runState
	trace              *TraceRecorder
	traceEntries       int
	traceDepth         int
	evaluations        *evaluationLog
	pendingBehaviors   []*ObjectBehavior
	heldBehaviors      mapState[*ObjectBehavior, bool]
	clockRun           *runState
	bodyCoroutinesMade int
}

// runStateCapture is one run's state by value: its budget spent, its notes, its
// scheduler's position and the calc usage evaluations of its open activations.
type runStateCapture struct {
	state           *runState
	steps, elements int64
	notes           []RunNote
	restoreSchedule func()
	calcUsageRuns   map[int64]map[calcUsageKey]*calcRun
}

// mapState is a map's identity and what it held at the mark, restored in place
// so every holder of the map sees the restored content.
type mapState[K comparable, V any] struct {
	live, saved map[K]V
}

func captureMap[K comparable, V any](m map[K]V) mapState[K, V] {
	return mapState[K, V]{live: m, saved: maps.Clone(m)}
}

// restore puts the saved content back into the live map and returns it, nil for
// a map that was nil at the mark.
func (s mapState[K, V]) restore() map[K]V {
	if s.live == nil {
		return nil
	}
	clear(s.live)
	maps.Copy(s.live, s.saved)
	return s.live
}

// Snapshot captures the run-derived state of the context between steps: its
// objects and their values, the bus, the clock, its run bookkeeping and the state
// of every behavior an object of it runs. An executor driven over the context is
// captured through its own Snapshot. It fails with ErrSnapshotMidRun from inside
// a step and with ErrSnapshotPausedBody while a body is paused mid-statement.
func (ctx *Context) Snapshot() (*Snapshot, error) {
	return ctx.snapshotWith(nil, nil)
}

// Snapshot captures the executor's state along with its context's (Context.Snapshot).
func (e *ActionExecutor) Snapshot() (*Snapshot, error) {
	return e.ctx.snapshotWith([]*ActionExecutor{e}, nil)
}

// Snapshot captures the executor's state along with its context's (Context.Snapshot).
func (e *StateExecutor) Snapshot() (*Snapshot, error) {
	return e.ctx.snapshotWith(nil, []*StateExecutor{e})
}

func (ctx *Context) snapshotWith(actions []*ActionExecutor, states []*StateExecutor) (*Snapshot, error) {
	if ctx.runDepth > 0 || ctx.actionDepth > 0 || ctx.calcDepth > 0 || ctx.pausable != nil || ctx.probes > 0 {
		return nil, ErrSnapshotMidRun
	}
	for _, behavior := range ctx.objectBehaviors {
		switch {
		case behavior.State != nil && !slices.Contains(states, behavior.State):
			states = append(states, behavior.State)
		case behavior.Action != nil && !slices.Contains(actions, behavior.Action):
			actions = append(actions, behavior.Action)
		}
	}
	s := &Snapshot{ctx: ctx, journal: ctx.markJournal(), run: ctx.captureRun()}
	s.captureRunState(ctx.run)
	s.captureRunState(ctx.clockRun.state)
	for _, exec := range actions {
		capture, err := exec.capture()
		if err != nil {
			return nil, err
		}
		s.actions = append(s.actions, capture)
		s.captureRunState(exec.driven.state)
	}
	for _, exec := range states {
		capture, err := exec.capture()
		if err != nil {
			return nil, err
		}
		s.states = append(s.states, capture)
		s.captureRunState(exec.driven.state)
	}
	ctx.journals++
	ctx.snapshots = append(ctx.snapshots, s)
	return s, nil
}

// Restore brings the run back to the snapshot: what was written since is undone,
// what was made since is abandoned, and every executor captured stands where it
// stood. Snapshots taken since are released. The snapshot stays live to be
// restored again. It panics on a released snapshot.
func (s *Snapshot) Restore() {
	if s.released {
		panic("runtime: Restore of a released snapshot")
	}
	ctx := s.ctx
	at := slices.Index(ctx.snapshots, s)
	for _, later := range ctx.snapshots[at+1:] {
		later.released = true
		ctx.journals--
	}
	ctx.snapshots = ctx.snapshots[:at+1]
	ctx.rollbackJournal(s.journal)
	s.run.restore(ctx)
	for _, capture := range s.runStates {
		capture.restore()
	}
	for _, capture := range s.actions {
		capture.restore()
	}
	for _, capture := range s.states {
		capture.restore()
	}
}

// Release ends the snapshot: it is no longer restorable, and the journal keeps
// nothing for it. Releasing twice does nothing.
func (s *Snapshot) Release() {
	if s.released {
		return
	}
	s.released = true
	ctx := s.ctx
	at := slices.Index(ctx.snapshots, s)
	ctx.snapshots = slices.Delete(ctx.snapshots, at, at+1)
	ctx.journals--
	if ctx.journals == 0 {
		// Snapshots release in any order; the last one out empties the journal.
		ctx.journalWrites, ctx.journalUndos = ctx.journalWrites[:0], ctx.journalUndos[:0]
	}
}

// markJournal marks where a change begins in the journal, with what it keeps by value.
func (ctx *Context) markJournal() journalMark {
	return journalMark{
		writes: len(ctx.journalWrites), undos: len(ctx.journalUndos),
		created: len(ctx.created), attached: len(ctx.objectBehaviors),
		messages:     slices.Clone(ctx.messages),
		clockNow:     ctx.clock.now,
		clockWaiters: slices.Clone(ctx.clock.waiters),
	}
}

// rollbackJournal undoes every change journaled since the mark: the feature
// values written, the other changes noted, the bus, the clock, the objects made
// and the behaviors attached. The journal is cut back to the mark.
func (ctx *Context) rollbackJournal(mark journalMark) {
	for i := len(ctx.journalWrites) - 1; i >= mark.writes; i-- {
		*ctx.journalWrites[i].fv = ctx.journalWrites[i].prior
	}
	ctx.journalWrites = ctx.journalWrites[:mark.writes]
	for i := len(ctx.journalUndos) - 1; i >= mark.undos; i-- {
		ctx.journalUndos[i]()
	}
	ctx.journalUndos = ctx.journalUndos[:mark.undos]
	ctx.messages = slices.Clone(mark.messages)
	ctx.abandonCreationSince(mark.created, mark.attached)
	ctx.clock.now, ctx.clock.waiters = mark.clockNow, slices.Clone(mark.clockWaiters)
}

func (ctx *Context) captureRun() runCapture {
	c := runCapture{
		ids: ctx.ids, nextID: ctx.ids.next,
		activations: ctx.activations, runs: ctx.runs,
		run:                ctx.run,
		trace:              ctx.trace,
		evaluations:        ctx.evaluations,
		pendingBehaviors:   slices.Clone(ctx.pendingBehaviors),
		heldBehaviors:      captureMap(ctx.heldBehaviors),
		clockRun:           ctx.clockRun.state,
		bodyCoroutinesMade: ctx.bodyCoroutinesMade,
	}
	if ctx.trace != nil {
		c.traceEntries, c.traceDepth = len(ctx.trace.entries), ctx.trace.depth
	}
	return c
}

// restore rewinds the run bookkeeping to the capture. Identities stay monotone
// across contexts: a sequence handed to another context since (AdoptIdentities)
// is kept, and one is rewound only past what every context sharing it holds.
func (c runCapture) restore(ctx *Context) {
	if ctx.ids == c.ids {
		c.ids.release(c.nextID)
	}
	ctx.activations, ctx.runs = c.activations, c.runs
	ctx.run = c.run
	ctx.trace = c.trace
	if c.trace != nil && len(c.trace.entries) > c.traceEntries {
		c.trace.entries = c.trace.entries[:c.traceEntries]
	}
	if c.trace != nil {
		c.trace.depth = c.traceDepth
	}
	ctx.evaluations = c.evaluations
	ctx.pendingBehaviors = slices.Clone(c.pendingBehaviors)
	ctx.heldBehaviors = c.heldBehaviors.restore()
	ctx.clockRun.state = c.clockRun
	ctx.bodyCoroutinesMade = c.bodyCoroutinesMade
}

// captureRunState captures a run's state once, however many executors share it.
func (s *Snapshot) captureRunState(state *runState) {
	if state == nil || slices.ContainsFunc(s.runStates, func(c runStateCapture) bool { return c.state == state }) {
		return
	}
	s.runStates = append(s.runStates, runStateCapture{
		state: state, steps: state.steps, elements: state.elements,
		notes:           slices.Clone(state.notes),
		restoreSchedule: state.scheduler.mark(),
		calcUsageRuns:   cloneCalcUsageRuns(state.calcUsageRuns),
	})
}

func (c runStateCapture) restore() {
	c.state.steps, c.state.elements = c.steps, c.elements
	c.state.notes = slices.Clone(c.notes)
	c.restoreSchedule()
	clear(c.state.calcUsageRuns)
	maps.Copy(c.state.calcUsageRuns, cloneCalcUsageRuns(c.calcUsageRuns))
}

func cloneCalcUsageRuns(runs map[int64]map[calcUsageKey]*calcRun) map[int64]map[calcUsageKey]*calcRun {
	cloned := make(map[int64]map[calcUsageKey]*calcRun, len(runs))
	for activation, evaluations := range runs {
		cloned[activation] = maps.Clone(evaluations)
	}
	return cloned
}

// actionCapture is an action executor's state by value: its tokens, the frames
// of its performances, and its step, sweep and breakpoint bookkeeping.
type actionCapture struct {
	exec              *ActionExecutor
	tokens            []Token
	state             ExecutionState
	nextTokenID       int64
	stepCount         int
	sweep, sweeps     uint64
	pausedAt          string
	released          bool
	pauses            int64
	steps, stepsSpent int64
	inRun             bool
	awaiting          *actionFrame
	firedBreakpoints  mapState[breakpointVisit, bool]
	driven            *runState
	frames            []frameCapture
}

func (e *ActionExecutor) capture() (actionCapture, error) {
	for _, token := range e.tokens {
		if token.body != nil {
			return actionCapture{}, fmt.Errorf("%w: token %d of %s at %s", ErrSnapshotPausedBody,
				token.ID, symbolText(e.action), ActionNodeName(token.Location))
		}
	}
	c := actionCapture{
		exec: e, tokens: slices.Clone(e.tokens), state: e.state,
		nextTokenID: e.nextTokenID, stepCount: e.stepCount, sweep: e.sweep, sweeps: e.sweeps,
		pausedAt: e.pausedAt, released: e.released, pauses: e.pauses,
		steps: e.steps, stepsSpent: e.stepsSpent, inRun: e.inRun, awaiting: e.awaiting,
		firedBreakpoints: captureMap(e.firedBreakpoints),
		driven:           e.driven.state,
	}
	for _, perf := range e.reachableFrames() {
		c.frames = append(c.frames, captureFrame(perf))
	}
	return c, nil
}

func (c actionCapture) restore() {
	e := c.exec
	e.tokens = slices.Clone(c.tokens)
	e.state, e.nextTokenID, e.stepCount, e.sweep, e.sweeps = c.state, c.nextTokenID, c.stepCount, c.sweep, c.sweeps
	e.pausedAt, e.released, e.pauses = c.pausedAt, c.released, c.pauses
	e.steps, e.stepsSpent, e.inRun, e.awaiting = c.steps, c.stepsSpent, c.inRun, c.awaiting
	e.firedBreakpoints = c.firedBreakpoints.restore()
	e.driven.state = c.driven
	for _, perf := range c.frames {
		perf.restore()
	}
}

// reachableFrames lists every performance the executor's run may still touch:
// the root's tree of latest performances, and those its tokens run in.
func (e *ActionExecutor) reachableFrames() []*actionFrame {
	seen := make(map[*actionFrame]bool)
	var frames []*actionFrame
	var visit func(perf *actionFrame)
	visit = func(perf *actionFrame) {
		for ; perf != nil && !seen[perf]; perf = perf.parent {
			seen[perf] = true
			frames = append(frames, perf)
			for _, sub := range perf.subactions {
				visit(sub)
			}
		}
	}
	visit(e.root)
	for _, token := range e.tokens {
		visit(token.frame)
	}
	visit(e.awaiting)
	return frames
}

// frameCapture is one performance's state by value; the maps it holds are
// restored in place, so a frame reading one of them as its own sees the restored values.
type frameCapture struct {
	perf   *actionFrame
	saved  actionFrame
	locals []mapState[string, Value]
	data   mapState[string, Value]
}

func captureFrame(perf *actionFrame) frameCapture {
	c := frameCapture{perf: perf, saved: *perf, data: captureMap(perf.data)}
	for _, local := range perf.locals {
		c.locals = append(c.locals, captureMap(local))
	}
	c.saved.outer = slices.Clone(perf.outer)
	c.saved.connections = slices.Clone(perf.connections)
	c.saved.features = maps.Clone(perf.features)
	c.saved.aliases = maps.Clone(perf.aliases)
	c.saved.outputs = slices.Clone(perf.outputs)
	c.saved.subactions = maps.Clone(perf.subactions)
	c.saved.pending = clonePending(perf.pending)
	c.saved.nested = cloneNested(perf.nested)
	c.saved.nodes = slices.Clone(perf.nodes)
	return c
}

func (c frameCapture) restore() {
	perf := c.perf
	*perf = c.saved
	perf.locals = nil
	for _, local := range c.locals {
		perf.locals = append(perf.locals, local.restore())
	}
	perf.data = c.data.restore()
	perf.outer = slices.Clone(c.saved.outer)
	perf.connections = slices.Clone(c.saved.connections)
	perf.features = maps.Clone(c.saved.features)
	perf.aliases = maps.Clone(c.saved.aliases)
	perf.outputs = slices.Clone(c.saved.outputs)
	perf.subactions = maps.Clone(c.saved.subactions)
	perf.pending = clonePending(c.saved.pending)
	perf.nested = cloneNested(c.saved.nested)
	perf.nodes = slices.Clone(c.saved.nodes)
}

func clonePending(pending map[ast.Node]map[string][]Value) map[ast.Node]map[string][]Value {
	if pending == nil {
		return nil
	}
	cloned := make(map[ast.Node]map[string][]Value, len(pending))
	for node, pins := range pending {
		clonedPins := make(map[string][]Value, len(pins))
		for pin, values := range pins {
			clonedPins[pin] = slices.Clone(values)
		}
		cloned[node] = clonedPins
	}
	return cloned
}

func cloneNested(nested map[ast.Node][]nestedDelivery) map[ast.Node][]nestedDelivery {
	if nested == nil {
		return nil
	}
	cloned := make(map[ast.Node][]nestedDelivery, len(nested))
	for node, deliveries := range nested {
		cloned[node] = slices.Clone(deliveries)
	}
	return cloned
}

// stateCapture is a state executor's state by value: its configuration, the
// values its states hold, its events, timers, change triggers and do actions.
type stateCapture struct {
	exec               *StateExecutor
	state              ExecutionState
	activeConfig       *StateConfiguration
	nextEventID        int64
	events             eventHeap
	stateData          mapState[string, Value]
	stateAttrs         map[*ast.StateNode]mapState[string, Value]
	stateVisits        []string
	stateStack         []*ast.StateNode
	history            map[*ast.StateNode]historyRecord
	deferred           []Event
	lastDispatch       *Dispatch
	lastEventAt        float64
	doActions          []doActionCapture
	machineExited      bool
	driven             *runState
	inRun              bool
	timerScheduled     mapState[*lower.Transition, bool]
	timeTriggerVerdict mapState[*lower.Transition, error]
	changeFired        mapState[*lower.Transition, bool]
	firingChange       *lower.Transition
	firingNotes        []RunNote
	changeRearmed      mapState[*lower.Transition, bool]
	changeWaits        []changeWait
}

// doActionCapture is one do action's progress: the behaviors it has still to run.
type doActionCapture struct {
	act     *doAction
	pending []lower.StateBehavior
}

func (e *StateExecutor) capture() (stateCapture, error) {
	c := stateCapture{
		exec: e, state: e.state, activeConfig: cloneConfiguration(e.activeConfig),
		nextEventID:        e.nextEventID,
		stateData:          captureMap(e.stateData),
		stateAttrs:         make(map[*ast.StateNode]mapState[string, Value], len(e.stateAttrs)),
		stateVisits:        slices.Clone(e.stateVisits),
		stateStack:         slices.Clone(e.stateStack),
		history:            make(map[*ast.StateNode]historyRecord, len(e.history)),
		deferred:           slices.Clone(e.deferred),
		lastDispatch:       cloneDispatch(e.lastDispatch),
		lastEventAt:        e.lastEventAt,
		machineExited:      e.machineExited,
		driven:             e.driven.state,
		inRun:              e.inRun,
		timerScheduled:     captureMap(e.timerScheduled),
		timeTriggerVerdict: captureMap(e.timeTriggerVerdict),
		changeFired:        captureMap(e.changeFired),
		firingChange:       e.firingChange,
		firingNotes:        slices.Clone(e.firingNotes),
		changeRearmed:      captureMap(e.changeRearmed),
		changeWaits:        slices.Clone(e.changeWaits),
	}
	if e.eventQueue != nil {
		c.events = slices.Clone(e.eventQueue.events)
	}
	for node, attrs := range e.stateAttrs {
		c.stateAttrs[node] = captureMap(attrs)
	}
	for node, record := range e.history {
		c.history[node] = historyRecord{child: record.child, regions: maps.Clone(record.regions)}
	}
	for _, act := range e.doActions {
		if act.run != nil {
			return stateCapture{}, fmt.Errorf("%w: do behavior of state %s of %s", ErrSnapshotPausedBody,
				getNodeName(act.state), symbolText(e.stateMachine))
		}
		c.doActions = append(c.doActions, doActionCapture{act: act, pending: slices.Clone(act.pending)})
	}
	return c, nil
}

func (c stateCapture) restore() {
	e := c.exec
	e.state, e.activeConfig, e.nextEventID = c.state, cloneConfiguration(c.activeConfig), c.nextEventID
	if e.eventQueue != nil {
		e.eventQueue.events = slices.Clone(c.events)
	}
	e.stateData = c.stateData.restore()
	if e.stateAttrs != nil {
		clear(e.stateAttrs)
		for node, attrs := range c.stateAttrs {
			e.stateAttrs[node] = attrs.restore()
		}
	}
	e.stateVisits, e.stateStack = slices.Clone(c.stateVisits), slices.Clone(c.stateStack)
	if e.history != nil {
		clear(e.history)
		for node, record := range c.history {
			e.history[node] = &historyRecord{child: record.child, regions: maps.Clone(record.regions)}
		}
	}
	e.deferred, e.lastDispatch, e.lastEventAt = slices.Clone(c.deferred), cloneDispatch(c.lastDispatch), c.lastEventAt
	e.doActions = e.doActions[:0]
	for _, act := range c.doActions {
		act.act.pending, act.act.run = slices.Clone(act.pending), nil
		e.doActions = append(e.doActions, act.act)
	}
	e.machineExited, e.driven.state, e.inRun = c.machineExited, c.driven, c.inRun
	e.timerScheduled = c.timerScheduled.restore()
	e.timeTriggerVerdict = c.timeTriggerVerdict.restore()
	e.changeFired = c.changeFired.restore()
	e.firingChange, e.firingNotes = c.firingChange, slices.Clone(c.firingNotes)
	e.changeRearmed = c.changeRearmed.restore()
	e.changeWaits = slices.Clone(c.changeWaits)
}

func cloneConfiguration(config *StateConfiguration) *StateConfiguration {
	if config == nil {
		return nil
	}
	return &StateConfiguration{simpleState: config.simpleState, regionStates: maps.Clone(config.regionStates)}
}

func cloneDispatch(dispatch *Dispatch) *Dispatch {
	if dispatch == nil {
		return nil
	}
	cloned := *dispatch
	cloned.Resumed = slices.Clone(dispatch.Resumed)
	return &cloned
}
