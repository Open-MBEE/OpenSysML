package runtime

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ErrAmbiguousSuccession reports a node whose flow could continue along more
// than one succession, which the token semantics do not resolve.
var ErrAmbiguousSuccession = errors.New("more than one succession is enabled")

// actionLabelPrefix opens the text naming an action in diagnostics and choices.
const actionLabelPrefix = "action "

// ActionExecutor executes action bodies using token-flow semantics.
type ActionExecutor struct {
	// performances holds the action's own performance, root, and runs its nodes' as
	// its subperformances; self is the object performing the action, whose
	// connections route what it sends.
	performances
	// action states the body performed; performed is the action as named, a usage
	// stating no body of its own or the action itself, whose metadata binds the performance.
	action, performed *symbols.Symbol
	// tool is the ToolExecution annotating performed, whose tool performs the action in
	// place of its body; nil for an action performed by its body.
	tool *toolExecution
	// occurrence is the action performance materialized for a performed usage. It
	// holds what the action's own features hold, and data mirrors it.
	occurrence *Instance
	graph      *lower.ActionGraph // Execution IR
	// features are the attributes and parameters the performance holds: those the
	// graph declares, then the inherited ones none of them redefines.
	features    []lower.Attribute
	tokens      []Token
	state       ExecutionState
	nextTokenID int64
	stepCount   int // Current step number for tracing
	breakpoints map[string]bool
	// firedBreakpoints records the token visits a breakpoint already stopped on.
	firedBreakpoints map[breakpointVisit]bool
	// sweep numbers the pass over a flow's tokens in progress, 0 between passes; sweeps
	// counts those begun. A token a sweep moved is not an arrival until the sweep ends.
	sweep, sweeps uint64
	inputs        map[string]Value // Input parameter bindings, applied over attribute defaults
	pausedAt      string           // Node name RunToCompletion stopped at, empty when it ran to the end
	// released is set once Release has ended the run for good.
	released bool
	// pauses counts the body pauses so far, ordering the paused runs' resumption.
	pauses int64

	// driven is this executor's run over however many calls drive it: begun once,
	// so the step budget is reset once, and keeping its scheduler throughout.
	driven executorRun
	// steps of the action's token-flow budget the current call has spent, by the
	// tokens of its flow and of the flows body statements run alike.
	steps int64
	// stepsSpent are the steps a drive of the clock took before waking this action,
	// counted against the same budget.
	stepsSpent int64
	// inRun is set while RunToCompletion drives the steps, whose budget they share.
	inRun bool
	// moved is set once a token acted — a failed step included — or the body wrote a
	// feature, and cleared when the start that attached the execution to its object settles.
	moved bool
	// awaiting is the subflow whose parked tokens a run waits on the clock for,
	// nil for the action's own.
	awaiting *actionFrame
}

// chargeActionStep spends one step of the action's token-flow budget
// (MaxActionStepsEnvVar), which the tokens of every flow of the run share.
func (e *ActionExecutor) chargeActionStep() error {
	if e.stepsSpent+e.steps >= e.ctx.maxActionSteps {
		return budgetExceeded(ErrActionStepLimitExceeded,
			fmt.Sprintf("execution exceeded max steps (%d steps; raise %s to allow more), possible infinite loop",
				e.ctx.maxActionSteps, MaxActionStepsEnvVar))
	}
	e.steps++
	return nil
}

// beginSweep opens a pass over a flow's tokens and returns the call that closes it,
// restoring the enclosing pass a body statement's flow runs within.
func (e *ActionExecutor) beginSweep() func() {
	outer := e.sweep
	e.sweeps++
	e.sweep = e.sweeps
	return func() { e.sweep = outer }
}

// breakpointVisit identifies one token's stay at one node.
type breakpointVisit struct {
	token int64
	node  ast.Node
}

// SetInputs binds input parameter values into the action's feature space.
// Inputs are applied after attribute defaults, so they override defaults with
// the same name. Must be called before initialize().
func (e *ActionExecutor) SetInputs(inputs map[string]Value) {
	e.inputs = inputs
}

// newActionExecutor creates an action executor. self is the object performing
// the action, nil for an action no object performs.
func newActionExecutor(ctx *Context, action *symbols.Symbol, self *Instance) (*ActionExecutor, error) {
	return newActionExecutorOf(ctx, action, action, self, nil)
}

// newActionExecutorOf creates an executor performing performed, the action as named, by
// running action's body; occurrence holds the performance's own features, nil without one.
func newActionExecutorOf(
	ctx *Context,
	performed, action *symbols.Symbol,
	self *Instance,
	occurrence *Instance,
) (*ActionExecutor, error) {
	if action.Kind != symbols.SymbolActionUsage && action.Kind != symbols.SymbolActionDef {
		return nil, fmt.Errorf("symbol %s is not an action", action.Name)
	}
	if err := ctx.checkPerformer(self); err != nil {
		return nil, err
	}

	// A usage stating no body of its own performs the body of the action it names — the
	// definition typing it — as a classifier behavior binding does; under a tool, none.
	action, tool, err := ctx.performanceBody(performed, action)
	if err != nil {
		return nil, err
	}
	graph, err := lowerPerformance(action, tool)
	if err != nil {
		return nil, err
	}
	return newActionExecutorOn(ctx, performed, action, tool, graph, self, occurrence), nil
}

// newActionExecutorOn is an execution of graph, the lowering of action, ready to
// begin at its root and attached to ctx's clock.
func newActionExecutorOn(
	ctx *Context,
	performed, action *symbols.Symbol,
	tool *toolExecution,
	graph *lower.ActionGraph,
	self, occurrence *Instance,
) *ActionExecutor {
	exec := &ActionExecutor{
		performances: performances{ctx: ctx, self: self},
		action:       action,
		performed:    performed,
		tool:         tool,
		occurrence:   occurrence,
		graph:        graph,
		tokens:       make([]Token, 0),
		state:        StateReady,
		nextTokenID:  1,
		breakpoints:  make(map[string]bool),

		firedBreakpoints: make(map[breakpointVisit]bool),
	}
	exec.features = exec.performanceFeatures()
	exec.root = exec.newRootFrame()
	exec.owner = exec
	ctx.clock.attach(exec)
	return exec
}

// lowerPerformance lowers what a performance of action runs, in the scope it was written
// in: its token flow, or under a tool only its own interface, the body never running.
func lowerPerformance(action *symbols.Symbol, tool *toolExecution) (*lower.ActionGraph, error) {
	if tool != nil {
		graph, err := lower.ToActionInterface(action.Decl, declScope(action))
		if err != nil {
			return nil, fmt.Errorf("lower action interface: %w", err)
		}
		return graph, nil
	}
	graph, err := lower.ToActionGraph(action.Decl, declScope(action))
	if err != nil {
		return nil, fmt.Errorf("lower action graph: %w", err)
	}
	lower.StartFlow(graph)
	return graph, nil
}

// performanceFeatures lists the graph's attributes, then the inherited ones none
// redefines with their declaring scope; library-contributed features stay behind the seam.
// An attribute valued by none of its own takes the default of the one it redefines.
func (e *ActionExecutor) performanceFeatures() []lower.Attribute {
	features := slices.Clone(e.graph.Attributes)
	declared := make(map[string]int, len(features))
	for i, attr := range features {
		declared[attr.Name] = i
	}
	for _, member := range e.ctx.model.semantics.MembersOf(e.action) {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || !lower.DeclaresNodeFeature(usage) || e.ctx.libraryDeclared(member) {
			continue
		}
		name, _ := ast.EffectiveName(usage)
		if name == "" {
			name = member.Name
		}
		if name == "" {
			continue
		}
		if i, ok := declared[name]; ok {
			if features[i].Value == nil && features[i].Node == ast.Node(usage) {
				features[i].Value, features[i].Scope = e.ctx.model.semantics.ParameterDefault(member)
			}
			continue
		}
		declared[name] = len(features)
		value, scope := e.ctx.model.semantics.ParameterDefault(member)
		features = append(features, lower.Attribute{
			Name: name, Direction: usage.Direction, IsResult: usage.IsResult, Value: value, Node: usage, Scope: scope,
		})
	}
	return features
}

// Step advances execution by one step for all active tokens.
// Safely handles token slice modifications (fork/join) by collecting indices first.
//
// A step moves each token at most once: a token the step created or moved — a
// fork's branch, the one a synchronized node performs with — has its first step
// in the next, so the tokens held at a node are all in before it performs.
//
// A token that reaches an accept with no message it can consume parks there
// rather than failing: the action is suspended until a matching message
// arrives. When a step moves nothing and at least one token is parked, the
// executor enters StateWaiting instead of reporting a deadlock — a caller
// driving Step itself (the REPL, or a state machine running in the same
// context) may still post the awaited message and step again, which resumes
// the parked token. RunToCompletion has no such caller, so it turns a step
// that leaves the executor waiting into ErrAcceptDeadlock.
//
// A step ends early, with the executor suspended, when a body a token runs is
// about to perform a node a breakpoint is set on (see SetBreakpoint): the token
// stays at its node with its work half done and no further token steps. The
// next step steps the other tokens first, then resumes it.
//
// Returns an error if a deadlock unrelated to accepts is detected (no progress
// made and nothing is waiting for a message).
func (e *ActionExecutor) Step() error {
	defer e.ctx.beginExecutorRun(&e.driven)()

	if e.released {
		return fmt.Errorf("%w: its run ended when it was let go of", ErrExecutorReleased)
	}

	if e.state == StateCompleted {
		return nil // Already completed
	}

	if e.state == StateReady {
		return fmt.Errorf("executor not initialized (call initialize first)")
	}

	if !e.inRun {
		e.steps = 0
	}

	// Stepping resumes a run a breakpoint suspended.
	if e.state == StateSuspended {
		e.state = StateRunning
		e.pausedAt = ""
	}

	// A waiting executor is asked again whether its parked tokens can proceed:
	// messages may have been posted since the step that parked them.
	if e.state == StateWaiting {
		e.state = StateRunning
	}

	// Snapshot token state before step (for deadlock detection)
	tokenCountBefore := len(e.tokens)
	tokenLocationsBefore := make([]ast.Node, len(e.tokens))
	for i, t := range e.tokens {
		tokenLocationsBefore[i] = t.Location
	}

	// Under a sweep the tokens whose work paused resume last, the longest paused
	// first, once every other token has had its step: one pausing again and again
	// does not hold the rest back. An exploring step picks among every token able
	// to act, paused work that can go on among them.
	var paused []int64
	eligible := oneMoveEligible
	if !e.ctx.scheduling().oneMove() {
		paused = e.pausedTokens()
		eligible = func(t Token) bool { return !t.drivenByBody() && t.body == nil }
	}
	defer e.beginSweep()()

	// Several tokens advanced in one step are a choice point the library leaves open.
	order := e.beginStepOrder()
	endWrites := e.beginStepWrites(e.stepCount + 1)

	schedule := e.scheduleTokens(&order, eligible)
	acted, err := e.stepTokens(schedule, paused, &order)
	if refused := e.ctx.scheduling().refusal(); refused != nil {
		err = refused
	}
	// What the tokens wrote, the order they took and that they acted are facts of
	// the step whether or not it failed.
	endWrites()
	e.noteTokenOrder(e.stepCount+1, order, schedule)
	if acted {
		e.moved = true
	}
	progressMade := e.tokensProgressed(tokenCountBefore, tokenLocationsBefore)
	if err != nil {
		e.endPausedBodies()
		return err
	}

	// A step a breakpoint ends leaves every other token where it was, yet the run
	// went on.
	if e.state == StateSuspended {
		progressMade = true
		e.moved = true
	}

	// If no progress and tokens remain, either the action is suspended waiting
	// for a message, or it is stuck for a reason no message can resolve.
	if !progressMade && len(e.tokens) > 0 {
		if !e.anyTokenWaiting() {
			return fmt.Errorf("%w: %d token(s) stuck, no progress made",
				ErrActionDeadlock, len(e.tokens))
		}
		e.state = StateWaiting
		if e.waitsOnClockAlone() {
			next, _ := e.NextWait()
			return fmt.Errorf("%w: %d token(s) wait on the clock, the earliest until t=%s",
				ErrNothingDue, len(e.visibleWaits()), semantics.FormatReal(next))
		}
	}

	// Increment step count
	e.stepCount++

	// Record trace after step completes
	if e.trace() != nil {
		e.trace().RecordActionStep(e.stepCount, e.tokens)
	}

	if e.state == StateCompleted {
		return e.ctx.endedWhole(&e.driven)
	}
	return nil
}

// tokensProgressed reports whether the tokens got anywhere since the count and locations
// given: one created or consumed, one at another node, or all consumed.
func (e *ActionExecutor) tokensProgressed(countBefore int, locationsBefore []ast.Node) bool {
	if len(e.tokens) != countBefore || len(e.tokens) == 0 {
		return true
	}
	for i := 0; i < len(e.tokens) && i < len(locationsBefore); i++ {
		if e.tokens[i].Location != locationsBefore[i] {
			return true
		}
	}
	return false
}

// waitsOnClockAlone reports whether every remaining token is parked on the clock
// for an instant it has not reached (or paused for work that is), so only advancing it moves the action.
func (e *ActionExecutor) waitsOnClockAlone() bool {
	for _, token := range e.tokens {
		if token.pausedOnClock() {
			if held := token.heldWaiter(); held != nil && held.dueWork() {
				return false
			}
			continue
		}
		if token.Wait == nil || !token.Wait.Timed || token.Wait.Due <= e.ctx.clock.now {
			return false
		}
	}
	return len(e.tokens) > 0
}

// anyTokenWaiting reports whether some token is parked at an accept, or paused
// for work of its own that waits on the clock.
func (e *ActionExecutor) anyTokenWaiting() bool {
	for _, token := range e.tokens {
		if token.Wait != nil || token.pausedOnClock() {
			return true
		}
	}
	return false
}

// waitingTokens returns the parked tokens of perf's flow (of the whole action for
// nil), in token-ID order, so that a report of what an action is waiting for does
// not depend on step scheduling.
func (e *ActionExecutor) waitingTokens(perf *actionFrame) []Token {
	waiting := make([]Token, 0, len(e.tokens))
	for _, token := range e.tokens {
		if token.Wait != nil && token.inFlowOf(perf) {
			waiting = append(waiting, token)
		}
	}
	sort.Slice(waiting, func(i, j int) bool { return waiting[i].ID < waiting[j].ID })
	return waiting
}

// deadlockError describes a suspension that can never end: the accepts still
// waiting in perf's flow (the action's for nil), and any token of it blocked for
// another reason alongside them.
func (e *ActionExecutor) deadlockError(perf *actionFrame) error {
	where := actionLabelPrefix + symbolText(e.action)
	if perf != nil {
		where = perf.describe()
	}
	return fmt.Errorf("%w in %s: nothing can post the awaited message (%s)",
		ErrAcceptDeadlock, where, e.describeWaits(perf))
}

// describeWaits lists what the parked tokens of perf's flow (the action's for nil) wait for.
func (e *ActionExecutor) describeWaits(perf *actionFrame) string {
	waiting := e.waitingTokens(perf)
	descriptions := make([]string, 0, len(waiting))
	for _, token := range waiting {
		descriptions = append(descriptions, token.Wait.String())
	}
	if blocked := len(e.tokensIn(perf)) - len(waiting); blocked > 0 {
		descriptions = append(descriptions,
			fmt.Sprintf("%d token(s) blocked for another reason", blocked))
	}
	return strings.Join(descriptions, "; ")
}

// RunToCompletion executes until StateCompleted, a breakpoint, or error.
// Includes infinite loop protection.
//
// A run stops as soon as a token sits on a node a breakpoint was set on
// (see SetBreakpoint), or a body a token runs is about to perform one, leaving
// the tokens where they are so the run can be resumed by calling RunToCompletion
// again or stepped with Step; PausedAt names the node it stopped at. With no
// breakpoints set the run is unconditional.
//
// Nothing outside the action can post a message while this runs, so an action
// whose every remaining token is parked at an accept for a message can never be
// resumed: the suspension is a deadlock and is reported as ErrAcceptDeadlock at
// the first step that makes no progress. A token parked on the clock is resumed
// by advancing it to its instant, running whatever else is due there too.
func (e *ActionExecutor) RunToCompletion() error {
	return e.run(false)
}

// run is the RunToCompletion loop, holding the clock where it is when
// atCurrentTime is set.
func (e *ActionExecutor) run(atCurrentTime bool) error {
	defer e.ctx.beginExecutorRun(&e.driven)()

	if e.released {
		return fmt.Errorf("%w: its run ended when it was let go of", ErrExecutorReleased)
	}

	e.steps = 0
	e.inRun = true
	defer func() { e.inRun = false }()

	e.pausedAt = ""
	if e.state == StateSuspended {
		e.state = StateRunning
	}

	// A run may start from StateWaiting: a caller that stepped an action into a
	// suspension and then posted the awaited message resumes it here.
	var progress dueProgress
	waits := func() bool { return e.state == StateWaiting && e.waitsOnClock(nil) && !e.canProceed(nil) }
	awaitsMessage := func() bool { return e.state == StateWaiting && !e.canProceed(nil) }
	for e.state == StateRunning || e.state == StateWaiting {
		// Tokens parked on the clock alone: advancing it is what moves them; a run
		// performing this action for a body pauses that body's run instead.
		if waits() {
			if atCurrentTime {
				return nil
			}
			if paused, err := e.ctx.pauseForClock(e, waits); err != nil {
				return err
			} else if paused {
				continue
			}
			if err := e.ctx.driveClock(e.describeWaits(nil)); err != nil {
				return err
			}
			moved, err := e.awaitClock(nil, &progress)
			if err != nil {
				return err
			}
			if moved {
				continue
			}
			break
		} else if !atCurrentTime && awaitsMessage() {
			// Tokens parked for a message: a do behavior pauses until one is in
			// flight or its state is left; any other run deadlocks below.
			if paused, err := e.ctx.pauseForMessage(e, awaitsMessage); err != nil {
				return err
			} else if paused {
				continue
			}
		}

		if node := e.breakpointHit(); node != "" {
			e.pausedAt = node
			e.state = StateSuspended
			return nil
		}

		if err := e.chargeActionStep(); err != nil {
			e.endPausedBodies()
			return err
		}

		if err := e.Step(); err != nil && !errors.Is(err, ErrNothingDue) {
			return err
		}
		if e.state == StateWaiting && (atCurrentTime || !e.waitsOnClock(nil)) {
			break
		}
	}
	if e.state == StateWaiting && !atCurrentTime {
		e.endPausedBodies()
		return e.deadlockError(nil)
	}
	return nil
}

// awaitClock runs what is due, then moves the clock to the earliest wait, until a
// parked token of perf's flow (the action's for nil) can proceed; false if it cannot move.
// Its turn in the due order is a poll of its own waits; one finding them still waiting settles it.
func (e *ActionExecutor) awaitClock(perf *actionFrame, progress *dueProgress) (bool, error) {
	awaiting := e.awaiting
	e.awaiting = perf
	defer func() { e.awaiting = awaiting }()
	// The step that parked the token got somewhere.
	progress.unsettle()
	for {
		picked, err := e.ctx.runDue(e, progress)
		if err != nil {
			return false, err
		}
		if e.canProceed(perf) {
			return true, nil
		}
		if picked {
			progress.settle(e)
			continue
		}
		if !e.ctx.advanceToNextDue(progress) {
			return false, nil
		}
	}
}

// dueNow reports whether a parked token of perf's flow (the action's for nil) can
// proceed now: its instant has come, its message is in flight, or its performed action has work due.
func (e *ActionExecutor) dueNow(perf *actionFrame) bool {
	return e.hasDueTimeWait(perf) || e.hasPendingSignal(perf) || e.hasDueHeldRun(perf)
}

// canProceed reports whether a parked token of perf's flow (the action's for nil)
// can proceed now: it is due, or the condition its accept waits on holds.
func (e *ActionExecutor) canProceed(perf *actionFrame) bool {
	return e.dueNow(perf) || e.changeWaitHolds(perf)
}

// changeWaitHolds reports a token of perf's flow (the action's for nil) parked at
// an accept whose condition holds now; one the step cannot evaluate counts, so the step reports it.
func (e *ActionExecutor) changeWaitHolds(perf *actionFrame) bool {
	for i := range e.tokens {
		token := &e.tokens[i]
		if token.Wait == nil || token.Wait.Timed || token.Wait.Trigger == "" || !token.inFlowOf(perf) {
			continue
		}
		usage, ok := token.Location.(*ast.Usage)
		if !ok {
			continue
		}
		accept, ok := e.graphOf(token.frame).Accepts[usage]
		if _, isChange := accept.Trigger.(*ast.ChangeEvent); !ok || !isChange {
			continue
		}
		if holds, err := e.triggerHolds(token, accept); err != nil || holds {
			return true
		}
	}
	return false
}

// hasDueHeldRun reports whether an action performed by the paused work of a token
// of perf's flow (the action's for nil) has work due at this instant.
func (e *ActionExecutor) hasDueHeldRun(perf *actionFrame) bool {
	for _, token := range e.tokens {
		if held := token.heldWaiter(); held != nil && token.inFlowOf(perf) && held.dueWork() {
			return true
		}
	}
	return false
}

// waitsOnClock reports whether a token of perf's flow (the action's for nil) is
// parked on the clock, or paused for work of its own that is.
func (e *ActionExecutor) waitsOnClock(perf *actionFrame) bool {
	if len(e.timeWaits(perf)) > 0 {
		return true
	}
	for _, token := range e.tokens {
		if token.pausedOnClock() && token.inFlowOf(perf) {
			return true
		}
	}
	return false
}

// RunToQuiescence runs the action until it completes, stops at a breakpoint, or
// parks every remaining token at an accept, holding the clock where it is: for an
// object's action a parked token is quiescence, not a deadlock.
func (e *ActionExecutor) RunToQuiescence() error {
	if err := e.run(true); err != nil && !errors.Is(err, ErrAcceptDeadlock) {
		return err
	}
	return nil
}

// HasPendingSignal reports whether a message in flight would let a parked token
// proceed, without consuming it.
func (e *ActionExecutor) HasPendingSignal() bool {
	return e.hasPendingSignal(nil)
}

// hasPendingSignal reports whether a message in flight would let a parked token
// of perf's flow (of the whole action for nil) proceed, without consuming it.
func (e *ActionExecutor) hasPendingSignal(perf *actionFrame) bool {
	pending := e.ctx.acceptable()
	return e.parkedAcceptTakes(perf, func(matches func(Message) bool, failed *error) bool {
		for _, msg := range pending {
			// A port that fails to resolve counts as pending: the step this
			// provokes surfaces the failure.
			if matches(msg) || *failed != nil {
				return true
			}
		}
		return false
	})
}

// acceptsMessage reports whether a token parked at a signal accept, or an action
// performed for a paused token, would take m; a port failing to resolve is the error.
func (e *ActionExecutor) acceptsMessage(m Message) (bool, error) {
	var err error
	if e.parkedAcceptTakes(nil, func(matches func(Message) bool, failed *error) bool {
		accepted := matches(m)
		err = *failed
		return accepted || err != nil
	}) {
		return err == nil, err
	}
	for _, token := range e.tokens {
		if held, ok := token.heldWaiter().(messageAcceptor); ok {
			if accepted, err := held.acceptsMessage(m); err != nil || accepted {
				return accepted, err
			}
		}
	}
	return false, nil
}

// parkedAcceptTakes calls takes with the predicate of each signal accept a token of
// perf's flow (of the whole action for nil) is parked at, until one reports true.
func (e *ActionExecutor) parkedAcceptTakes(perf *actionFrame, takes func(matches func(Message) bool, failed *error) bool) bool {
	for _, token := range e.tokens {
		if token.Wait == nil || token.Wait.Timed || !token.inFlowOf(perf) {
			continue
		}
		usage, ok := token.Location.(*ast.Usage)
		if !ok {
			continue
		}
		accept, isAccept := e.graphOf(token.frame).Accepts[usage]
		if !isAccept || accept.Trigger != nil {
			continue
		}
		if takes(e.acceptMatch(token.frame, accept, usage)) {
			return true
		}
	}
	return false
}

// acceptMatch is the predicate an accept node holds a message to: it reaches
// the node, and conforms to the type the accept names or carries the event it
// subsets, read over the performance's data. The signal is tested first, so only a
// message of the accept's own signal resolves the `via` port; if that fails the
// predicate matches nothing and the failure is left in the returned error slot.
func (e *ActionExecutor) acceptMatch(frame *actionFrame, accept lower.Accept, usage *ast.Usage) (func(Message) bool, *error) {
	var failed error
	ec := e.evalContextFor(frame, accept.Scope)
	return func(m Message) bool {
		if failed != nil {
			return false
		}
		if accept.SignalType != nil {
			if !e.ctx.messageMatches(m, accept.SignalType, accept.Scope) {
				return false
			}
		} else if !ec.carriesEvent(m, accept.SubsetsEvent) {
			return false
		}
		reaches, err := e.ctx.messageReaches(m, ActionNodeName(usage), accept.ViaPort, e.self)
		if err != nil {
			failed = err
		}
		return err == nil && reaches
	}, &failed
}

// breakpointHit returns the name of a breakpoint node a token sits on and has not yet
// stopped the run at, or "" if none does. Firing once per token and visit means a resumed
// run continues past the node it stopped at, while a token that comes back around a loop
// stops again; tokens held at a synchronized node stop once, when the last has arrived.
func (e *ActionExecutor) breakpointHit() string {
	if len(e.breakpoints) == 0 {
		return ""
	}
	for visit := range e.firedBreakpoints {
		if loc, ok := e.tokenLocation(visit.token); !ok || loc != visit.node {
			delete(e.firedBreakpoints, visit)
		}
	}
	for i, token := range e.tokens {
		name := e.breakpointNameOf(token.Location)
		if name == "" {
			continue
		}
		if e.firedBreakpoints[breakpointVisit{token: token.ID, node: token.Location}] {
			continue
		}
		// A synchronized node stops once, for the arrivals its next performance collapses.
		performers := []int{i}
		if consumed, held := e.arrivals(token); held {
			if !slices.Contains(consumed, i) {
				continue
			}
			performers = consumed
		}
		if e.firedBreakpoints == nil {
			e.firedBreakpoints = make(map[breakpointVisit]bool)
		}
		for _, idx := range performers {
			e.firedBreakpoints[breakpointVisit{token: e.tokens[idx].ID, node: token.Location}] = true
		}
		return name
	}
	return ""
}

// tokenLocation returns where the given token sits, if it is still active.
func (e *ActionExecutor) tokenLocation(id int64) (ast.Node, bool) {
	for _, token := range e.tokens {
		if token.ID == id {
			return token.Location, true
		}
	}
	return nil, false
}

// breakpointNameOf returns the name a breakpoint is set on for the given node,
// or "" when none is. A node answers to its short name as well as its name.
func (e *ActionExecutor) breakpointNameOf(node ast.Node) string {
	for _, name := range ActionNodeNames(node) {
		if e.breakpoints[name] {
			return name
		}
	}
	return ""
}

// PausedAt returns the breakpoint node the last run stopped at, or "" when the
// run was not stopped by a breakpoint.
func (e *ActionExecutor) PausedAt() string {
	return e.pausedAt
}

// ActionNodeName returns the declared name of an action graph node, or "" when
// the node is anonymous or not a named node kind.
func ActionNodeName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.InitialNode:
		return n.Name
	case *ast.FinalNode:
		return "done"
	case *ast.ForkNode:
		return n.Name
	case *ast.JoinNode:
		return n.Name
	case *ast.MergeNode:
		return n.Name
	case *ast.DecisionNode:
		return n.Name
	case *ast.ActionExecutionNode:
		return n.Name
	case *ast.StateNode:
		return n.Name
	case *ast.Usage:
		if name, _ := ast.EffectiveName(n); name != "" {
			return name
		}
		return n.Ident.ShortName
	case *ast.Definition:
		if n.Ident.Name != "" {
			return n.Ident.Name
		}
		return n.Ident.ShortName
	default:
		return ""
	}
}

// ActionNodeNames returns every name a node answers to: its name and, for a
// usage, its declared short name, which is a name of its own.
func ActionNodeNames(node ast.Node) []string {
	name := ActionNodeName(node)
	var names []string
	if name != "" {
		names = append(names, name)
	}
	var short string
	switch n := node.(type) {
	case *ast.Usage:
		short = n.Ident.ShortName
	case *ast.Definition:
		short = n.Ident.ShortName
	}
	if short != "" && short != name {
		names = append(names, short)
	}
	return names
}

// NodeNames returns the names of the action's graph nodes, in declaration
// order, then those of the flows nested under it: the flows its nodes own and
// the block flows their bodies state. Anonymous nodes are omitted; a debugger
// uses it to check that a breakpoint names a node that exists.
func (e *ActionExecutor) NodeNames() []string {
	names := make([]string, 0, len(e.graph.Nodes))
	for _, node := range e.graph.Nodes {
		names = append(names, ActionNodeNames(node)...)
	}
	return append(names, e.subflowNodeNames(e.graph)...)
}

// initializeAttributes fills the features no supplied input holds: from the occurrence's
// slots, else the declared defaults in order, each evaluated where it was declared.
func (e *ActionExecutor) initializeAttributes() error {
	if e.occurrence != nil {
		for _, attr := range e.features {
			if _, held := e.root.data[e.root.key(attr.Name)]; held {
				continue
			}
			fv, err := e.occurrence.GetFeatureValue(e.ctx, attr.Name)
			if err != nil {
				return fmt.Errorf("%w: read %s of object #%d: %w",
					ErrActionPerformanceOccurrence, attr.Name, e.occurrence.ID, err)
			}
			if value := fv.HeldValue(); value.Kind != ValInvalid {
				e.root.data[e.root.key(attr.Name)] = value
			}
		}
		return nil
	}

	ec := e.evalContextFor(e.root, e.graph.Scope)
	defer ec.beginStep()()
	for _, attr := range e.features {
		if attr.Value == nil {
			continue
		}
		if _, held := e.root.data[e.root.key(attr.Name)]; held {
			continue
		}
		value, err := ec.evalIn(attr.Scope).Eval(attr.Value)
		if err != nil {
			return fmt.Errorf("eval attribute default %s: %w", attr.Name, err)
		}
		e.root.data[e.root.key(attr.Name)] = value
	}

	return nil
}

// declaresAttribute reports whether the action declares an attribute of this
// name, which its performance holds rather than the object performing it.
func (e *ActionExecutor) declaresAttribute(name string) bool {
	for _, attr := range e.features {
		if attr.Name == name {
			return true
		}
	}
	return false
}

// assignAround holds nothing: an action's performance is the outermost its nodes reach.
func (e *ActionExecutor) assignAround(string, Value) (bool, error) {
	return false, nil
}

// runOwnFlow runs the flow a block-declared node states of its own to completion.
func (e *ActionExecutor) runOwnFlow(perf *actionFrame) error {
	return e.runSubflow(perf)
}

// setFeature writes into the action's feature space, through the performance
// occurrence for a feature the action declares: the occurrence is authoritative
// for those, and data mirrors what it holds after the write.
func (e *ActionExecutor) setFeature(name string, value Value) error {
	if e.occurrence != nil && e.declaresAttribute(name) {
		if err := e.occurrence.SetFeatureValue(e.ctx, name, value); err != nil {
			return fmt.Errorf("%w: write %s of object #%d: %w",
				ErrActionPerformanceOccurrence, name, e.occurrence.ID, err)
		}
		fv, err := e.occurrence.GetFeatureValue(e.ctx, name)
		if err != nil {
			return fmt.Errorf("%w: read %s of object #%d after write: %w",
				ErrActionPerformanceOccurrence, name, e.occurrence.ID, err)
		}
		value = fv.HeldValue()
	} else if err := e.ctx.checkNamedWrite(e.graph.Scope, actionLabelPrefix+symbolText(e.action), name, &value); err != nil {
		// No occurrence holds this feature, so its declaration is checked here
		// rather than by the write to that occurrence.
		return err
	}
	e.root.data[e.root.key(name)] = value
	e.moved = true
	return nil
}

// setFrameFeatures writes several values into a performance, in name order so a
// failure among them is the same one however they were collected.
func (e *ActionExecutor) setFrameFeatures(frame *actionFrame, values map[string]Value) error {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := e.setFrameFeature(frame, name, values[name]); err != nil {
			return err
		}
	}
	return nil
}

// hasFlow reports whether the action states a flow to start: an action with no
// step performs none, while one whose steps give no start fails to initialize.
func (e *ActionExecutor) hasFlow() bool {
	return e.graph != nil && (e.graph.Initial != nil || statesSteps(e.graph))
}

// statesSteps reports whether the graph has a step to perform, a final node aside.
func statesSteps(graph *lower.ActionGraph) bool {
	for _, node := range graph.Nodes {
		if _, final := node.(*ast.FinalNode); !final {
			return true
		}
	}
	return false
}

// noFlowStart says why a flow that states steps has no step to start at.
func noFlowStart(graph *lower.ActionGraph) string {
	if !statesSteps(graph) {
		return ""
	}
	_, err := lower.CaseFlowStart(graph)
	return ": " + err.Error()
}

// completeWithoutFlow completes an action stating no flow: it performs no step,
// so its performance begins, takes its inputs, and ends at once.
func (e *ActionExecutor) completeWithoutFlow() error {
	if err := e.checkResultParameters(); err != nil {
		return err
	}
	e.ctx.beginPerformanceLife(e.occurrence, e.ctx.newActivation())
	if err := e.bindInputs(); err != nil {
		return err
	}
	e.state = StateCompleted
	e.ctx.endPerformanceLife(e.occurrence)
	return nil
}

// bindInputs writes the supplied inputs into the performance, then the
// attributes it declares: a default written in terms of an input reads it.
func (e *ActionExecutor) bindInputs() error {
	if err := e.fixWitnessInputs(); err != nil {
		return err
	}
	if err := e.checkInputNames(); err != nil {
		return err
	}
	if err := e.setFrameFeatures(e.root, e.inputs); err != nil {
		return err
	}
	if err := e.initializeAttributes(); err != nil {
		return fmt.Errorf("initialize attributes: %w", err)
	}
	return nil
}

// fixWitnessInputs takes the inputs the run's replayed witness fixes, when this
// performance begins the run, ahead of the caller's inputs and the defaults.
func (e *ActionExecutor) fixWitnessInputs() error {
	witness := e.ctx.scheduling().witnessInputs()
	if len(witness) == 0 {
		return nil
	}
	ec := e.evalContextFor(e.root, e.graph.Scope)
	defer ec.beginStep()()
	inputs := maps.Clone(e.inputs)
	if inputs == nil {
		inputs = make(map[string]Value, len(witness))
	}
	for _, in := range witness {
		dir, declared := e.parameterDirection(in.Feature)
		switch {
		case dir == ast.DirOut:
			return &WitnessInputError{Feature: in.Feature,
				Reason: fmt.Sprintf("action %s writes it back rather than reading it", symbolText(e.action))}
		case !declared && !e.declaresAttribute(in.Feature):
			return &WitnessInputError{Feature: in.Feature,
				Reason: fmt.Sprintf("action %s declares no such feature", symbolText(e.action))}
		}
		value := in.Value
		if value.Kind == ValInvalid {
			expr, ok := parseOneExpression("<witness>", in.Written)
			if !ok {
				return &WitnessInputError{Feature: in.Feature, Reason: fmt.Sprintf("%q is not an expression the notation reads", in.Written)}
			}
			evaluated, err := ec.Eval(expr)
			if err != nil {
				return &WitnessInputError{Feature: in.Feature, Reason: fmt.Sprintf("%q does not evaluate: %v", in.Written, err)}
			}
			value = evaluated
		}
		inputs[in.Feature] = value
	}
	e.inputs = inputs
	return nil
}

// parameterDirection reports the direction of the parameter of this name, and
// whether the action declares one.
func (e *ActionExecutor) parameterDirection(name string) (ast.FeatureDirection, bool) {
	if e.action == nil {
		return ast.DirNone, false
	}
	for _, param := range e.ctx.actionParametersOf(e.action) {
		if param.Name == name {
			return param.Direction, true
		}
	}
	return ast.DirNone, false
}

// checkInputNames reports a supplied input the caller cannot write: one naming
// no feature of the action, or a parameter the action only writes back (`out`).
func (e *ActionExecutor) checkInputNames() error {
	var unknown, outputs []string
	for name := range e.inputs {
		dir, declared := e.parameterDirection(name)
		switch {
		case declared && (dir == ast.DirIn || dir == ast.DirInOut):
		case dir == ast.DirOut:
			outputs = append(outputs, name)
		case !e.declaresAttribute(name):
			unknown = append(unknown, name)
		}
	}
	if len(outputs) > 0 {
		sort.Strings(outputs)
		return fmt.Errorf("%w: action %s writes %s back rather than reading it",
			ErrOutputActionInput, symbolText(e.action), strings.Join(outputs, ", "))
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("%w: action %s declares no %s",
		ErrUnknownActionInput, symbolText(e.action), strings.Join(unknown, ", "))
}

// initialize spawns initial token at InitialNode.
func (e *ActionExecutor) initialize() error {
	defer e.ctx.beginExecutorRun(&e.driven)()

	if e.graph.Initial == nil {
		return fmt.Errorf("%w: no initial node found in action %s%s",
			ErrInvalidActionFlow, e.action.Name, noFlowStart(e.graph))
	}

	// A nested node's own flow is validated here, not at construction, so a
	// malformed one is a typed error rather than a leaf that silently runs.
	if err := e.validateSubflows(e.graph); err != nil {
		return err
	}
	if err := e.checkResultParameters(); err != nil {
		return err
	}

	initialNode := e.graph.Initial
	e.ctx.beginPerformanceLife(e.occurrence, e.ctx.newActivation())
	if err := e.bindInputs(); err != nil {
		return err
	}

	// Spawn initial token
	token := Token{
		ID:       e.nextTokenID,
		Location: initialNode,
		frame:    e.root,
	}
	e.nextTokenID++
	e.tokens = append(e.tokens, token)

	e.state = StateRunning
	return nil
}

// stepToken advances a specific token by index.
func (e *ActionExecutor) stepToken(tokenIdx int) error {
	if tokenIdx < 0 || tokenIdx >= len(e.tokens) {
		return fmt.Errorf("invalid token index %d", tokenIdx)
	}
	defer e.beginTokenStep(e.tokens[tokenIdx].ID)()

	if e.tokens[tokenIdx].body != nil {
		return e.resumeBody(tokenIdx)
	}
	tokenIdx, ready := e.synchronize(tokenIdx)
	if !ready {
		return nil
	}
	token := &e.tokens[tokenIdx]

	switch node := token.Location.(type) {
	case *ast.InitialNode:
		return e.stepInitialNode(tokenIdx)
	case *ast.FinalNode:
		return e.stepFinalNode(tokenIdx)
	case *ast.ForkNode:
		return e.stepForkNode(tokenIdx)
	case *ast.JoinNode:
		return e.stepJoinNode(tokenIdx)
	case *ast.MergeNode:
		return e.stepMergeNode(tokenIdx)
	case *ast.DecisionNode:
		return e.stepDecisionNode(tokenIdx)
	case *ast.ActionExecutionNode:
		return e.stepActionExecutionNode(tokenIdx)
	case *ast.Usage:
		// Nested action invocation, or a nested case performed as a step
		if node.Kind == ast.UsageAction || lower.IsCaseNode(node) {
			return e.stepNestedAction(tokenIdx)
		}
		return fmt.Errorf("unsupported usage kind in action: %v", node.Kind)
	case *ast.WhileLoopActionNode, *ast.IfActionNode, *ast.AssignmentActionNode,
		*ast.SendStatement, *ast.TerminateStatement:
		// An action node member written as a statement (`then send x via p;`,
		// `then loop action { … } until c;`): lowering gave it the one statement
		// it was written as for a body, and a succession put it in the flow.
		return e.stepStatementNode(tokenIdx)
	default:
		return fmt.Errorf("unsupported node type: %T", node)
	}
}

// synchronize holds a token at a node several successions reach until one token has arrived
// over each; the earliest per succession then collapse into the one token that performs it.
func (e *ActionExecutor) synchronize(tokenIdx int) (int, bool) {
	token := e.tokens[tokenIdx]
	consumed, held := e.arrivals(token)
	if !held {
		return tokenIdx, true
	}
	if consumed == nil {
		return tokenIdx, false
	}

	sort.Sort(sort.Reverse(sort.IntSlice(consumed)))
	for _, idx := range consumed {
		e.removeToken(idx)
	}
	token.frame.live -= len(consumed) - 1
	e.tokens = append(e.tokens, Token{ID: e.nextTokenID, Location: token.Location, moved: e.sweep, frame: token.frame})
	e.nextTokenID++
	return len(e.tokens) - 1, true
}

// arrivals returns the tokens the next performance of the node token is held at collapses,
// the earliest per awaited succession, or nil while one has yet to deliver; held is false
// for a token no synchronization holds.
func (e *ActionExecutor) arrivals(token Token) (consumed []int, held bool) {
	if token.Via == (lower.ActionEdge{}) || token.body != nil || !synchronizes(token.Location) {
		return nil, false
	}
	_, join := token.Location.(*ast.JoinNode)
	incoming := e.awaitedSuccessions(token.frame, token.Location)
	if len(incoming) < 2 && !join {
		return nil, false
	}
	consumed = make([]int, 0, len(incoming))
	for _, edge := range incoming {
		idx, ok := e.arrival(token.frame, token.Location, edge, true)
		if !ok {
			return nil, true
		}
		consumed = append(consumed, idx)
	}
	return consumed, true
}

// synchronizes reports whether a node waits for all its incoming successions: every node
// does but a merge, the one multi-incoming node that fires per arrival (Actions::MergeAction).
func synchronizes(node ast.Node) bool {
	_, merge := node.(*ast.MergeNode)
	return !merge
}

// arrival returns the earliest token of frame held at node that arrived over edge; settled
// passes over those the sweep in progress moved there, which arrive once it ends.
func (e *ActionExecutor) arrival(frame *actionFrame, node ast.Node, edge lower.ActionEdge, settled bool) (int, bool) {
	for i, t := range e.tokens {
		if t.Location == node && t.frame == frame && t.body == nil && t.Via == edge &&
			(!settled || !e.moving(t)) {
			return i, true
		}
	}
	return 0, false
}

// moving reports whether the sweep in progress moved or created the token.
func (e *ActionExecutor) moving(t Token) bool {
	return e.sweep != 0 && t.moved == e.sweep
}

// awaitedSuccessions returns the successions into node a performance of it follows: every one
// of a join (its sources are 1..1), else those delivered or whose source a token may still perform.
func (e *ActionExecutor) awaitedSuccessions(frame *actionFrame, node ast.Node) []lower.ActionEdge {
	graph := e.graphOf(frame)
	incoming := incomingEdges(graph, node)
	if _, join := node.(*ast.JoinNode); join || len(incoming) < 2 {
		return incoming
	}
	live := e.reachableFrom(frame, node)
	awaited := make([]lower.ActionEdge, 0, len(incoming))
	for _, edge := range incoming {
		if _, delivered := e.arrival(frame, node, edge, false); delivered || live[edge.Source] {
			awaited = append(awaited, edge)
		}
	}
	return awaited
}

// reachableFrom returns the nodes of frame's flow the tokens of that flow not held at node may
// reach before node performs: paths through node itself or through a join it must feed do not count.
func (e *ActionExecutor) reachableFrom(frame *actionFrame, node ast.Node) map[ast.Node]bool {
	graph := e.graphOf(frame)
	reached := make(map[ast.Node]bool)
	for _, t := range e.tokens {
		if at, ok := t.positionIn(frame); ok && at != node {
			reached[at] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for current := range reached {
			if current == node || !e.leaves(frame, current, reached) {
				continue
			}
			for _, edge := range graph.Edges[current] {
				if !reached[edge.Target] {
					reached[edge.Target] = true
					changed = true
				}
			}
		}
	}
	return reached
}

// leaves reports whether a token may leave node given the nodes reached so far: a join is left
// only once every incoming succession has delivered or has a reachable source.
func (e *ActionExecutor) leaves(frame *actionFrame, node ast.Node, reached map[ast.Node]bool) bool {
	if _, join := node.(*ast.JoinNode); !join {
		return true
	}
	for _, edge := range incomingEdges(e.graphOf(frame), node) {
		if _, delivered := e.arrival(frame, node, edge, false); !delivered && !reached[edge.Source] {
			return false
		}
	}
	return true
}

// incomingEdges returns the successions into node, in the declaration order of
// the nodes they leave.
func incomingEdges(graph *lower.ActionGraph, node ast.Node) []lower.ActionEdge {
	var incoming []lower.ActionEdge
	for _, source := range graph.Nodes {
		for _, edge := range graph.Edges[source] {
			if edge.Target == node {
				incoming = append(incoming, edge)
			}
		}
	}
	return incoming
}

// Awaiting returns the successions into the node token is held at that no token has
// arrived over yet, nil for a token not held there.
func (e *ActionExecutor) Awaiting(token Token) []lower.ActionEdge {
	if token.Via == (lower.ActionEdge{}) || token.body != nil || !synchronizes(token.Location) {
		return nil
	}
	var awaited []lower.ActionEdge
	for _, edge := range e.awaitedSuccessions(token.frame, token.Location) {
		if _, delivered := e.arrival(token.frame, token.Location, edge, false); !delivered {
			awaited = append(awaited, edge)
		}
	}
	return awaited
}

// enabledSuccessions returns the successions a token at node may take, in
// declaration order: a guard that does not hold leaves no link to pass along
// (TransitionPerformance::transitionLink is HappensBefore[0..1]), so it is pruned.
func (e *ActionExecutor) enabledSuccessions(frame *actionFrame, node ast.Node) ([]lower.ActionEdge, error) {
	graph := frame.graph
	declared := graph.Edges[node]
	if len(declared) == 0 {
		return declared, nil
	}

	ec := e.evalContextFor(frame, graph.Scope)
	defer ec.beginStep()()

	enabled := make([]lower.ActionEdge, 0, len(declared))
	for _, edge := range declared {
		holds, err := e.guardHolds(ec, node, edge.Guard)
		if err != nil {
			return nil, err
		}
		if holds {
			enabled = append(enabled, edge)
		}
	}
	return enabled, nil
}

// guardHolds evaluates the guard a succession out of node carries; a succession
// carrying none is unconditional.
func (e *ActionExecutor) guardHolds(ec *EvalContext, node, guard ast.Node) (bool, error) {
	result, err := guardResult(ec, guard)
	if err != nil {
		return false, fmt.Errorf("eval guard of %s: %w", nodeDescription(node), err)
	}
	if !result.isBool() {
		return false, fmt.Errorf("%w: %s: guard must evaluate to boolean, got %v",
			ErrTypeMismatch, nodeDescription(node), result.Kind)
	}
	return result.Const.Bool, nil
}

// guardResult is what a guard evaluates to; no guard is true.
func guardResult(ec *EvalContext, guard ast.Node) (Value, error) {
	if guard == nil {
		return boolValue(true), nil
	}
	return ec.Eval(guard)
}

// probeGuard reads the guard of the succession at position i out of a decision
// node whose branch is already decided, as a probe the context undoes whole: the
// read reports a choice and leaves the run as it was. A guard with no result is
// noted and not selected.
func (e *ActionExecutor) probeGuard(frame *actionFrame, node *ast.DecisionNode, successors []lower.ActionEdge, i int) bool {
	result, err := func() (Value, error) {
		defer e.ctx.beginProbe()()
		ec := e.evalContextFor(frame, e.graphOf(frame).Scope)
		defer ec.beginStep()()
		return guardResult(ec, successors[i].Guard)
	}()
	if err == nil && !result.isBool() {
		err = fmt.Errorf("%w: guard must evaluate to boolean, got %v", ErrTypeMismatch, result.Kind)
	}
	if err != nil {
		e.noteUnevaluableGuard(frame, node, successors, i, err)
		return false
	}
	return result.Const.Bool
}

// scheduleTokens hands the step the tokens it may move, those eligible now, in
// the order the run's scheduling policy has it try them.
func (e *ActionExecutor) scheduleTokens(order *stepOrder, eligible func(Token) bool) *tokenSchedule {
	return e.ctx.scheduling().scheduleStep(e.stepCandidates(order, eligible))
}

// oneMoveEligible is the eligibility of a step moving one token: a token not
// driven by a body, whose own paused work, if any, would go on.
func oneMoveEligible(t Token) bool { return !t.drivenByBody() && (t.body == nil || t.resumable()) }

// stepCandidates lists the tokens a step may move, as the policy is handed them.
func (e *ActionExecutor) stepCandidates(order *stepOrder, eligible func(Token) bool) stepTokens {
	tokens := stepTokens{
		owner:  e,
		step:   e.stepCount + 1,
		ids:    make([]int64, 0, len(e.tokens)),
		parked: make(map[int64]bool),
		held:   make(map[int64]bool),
		label:  e.tokenLabel,
	}
	tokens.enabled = func(id int64) bool { return e.enabled(id, eligible) }
	for i, t := range e.tokens {
		if !e.moving(t) && eligible(t) {
			tokens.ids = append(tokens.ids, t.ID)
			if e.parked(t, order) {
				tokens.parked[t.ID] = true
			}
			if order.unready[t.ID] || e.fused(i) {
				tokens.held[t.ID] = true
			}
		}
	}
	return tokens
}

// fused reports whether the token at index i collapses into a synchronization
// another held token performs, the earliest arrival standing for both.
func (e *ActionExecutor) fused(i int) bool {
	consumed, held := e.arrivals(e.tokens[i])
	return held && consumed != nil && slices.Contains(consumed, i) && slices.Min(consumed) != i
}

// enabled reports whether the token would act were it stepped now: it is still
// eligible, and an accept it sits at is answered — or fails, which stepping it
// raises as the typed error — under a readiness probe that leaves the run as it was.
func (e *ActionExecutor) enabled(id int64, eligible func(Token) bool) bool {
	ready, _ := e.readiness(id, eligible)
	return ready
}

// readiness is enabled along with the typed error a failing accept would raise.
func (e *ActionExecutor) readiness(id int64, eligible func(Token) bool) (ready bool, fails error) {
	i := e.tokenIndex(id)
	if i < 0 || e.moving(e.tokens[i]) || !eligible(e.tokens[i]) {
		return false, nil
	}
	t := e.tokens[i]
	usage, ok := t.Location.(*ast.Usage)
	if !ok || t.body != nil {
		return true, nil
	}
	accept, isAccept := e.graphOf(t.frame).Accepts[usage]
	if !isAccept {
		return true, nil
	}
	defer e.ctx.beginProbe()()
	if accept.Trigger != nil {
		// triggerHolds may park the token's copy; the probe asks, it does not park.
		holds, err := e.triggerHolds(&t, accept)
		return holds || err != nil, err
	}
	pending := e.ctx.acceptable()
	if len(pending) == 0 {
		return false, nil
	}
	matches, failed := e.acceptMatch(t.frame, accept, usage)
	return slices.ContainsFunc(pending, matches) || *failed != nil, *failed
}

// tokenLabel names a token as the trace does, by ID and node.
func (e *ActionExecutor) tokenLabel(id int64) string {
	if loc, ok := e.tokenLocation(id); ok {
		return fmt.Sprintf("%d@%s", id, nodeIdentifier(loc))
	}
	return fmt.Sprintf("%d", id)
}

// parked reports whether the token cannot act by itself this step: held at a join
// or at an accept no message in flight answers. It still gets its turn.
func (e *ActionExecutor) parked(t Token, order *stepOrder) bool {
	if order.unready[t.ID] {
		return true
	}
	_, waitsForMessage := e.messageAccept(t)
	return waitsForMessage && !order.offered[t.ID]
}

// stepTokens gives each scheduled token its step, then the tokens whose paused
// work a sweep resumes last; a breakpoint on the way ends the sweep. It reports
// whether any token acted, one whose step failed among them.
func (e *ActionExecutor) stepTokens(schedule *tokenSchedule, paused []int64, order *stepOrder) (acted bool, err error) {
	for id, ok := schedule.Next(); ok; id, ok = schedule.Next() {
		if e.state == StateSuspended {
			break
		}
		// The token may have been removed by a join or final node.
		i := e.tokenIndex(id)
		if i < 0 || e.moving(e.tokens[i]) || e.tokens[i].drivenByBody() {
			schedule.Acted(id, false)
			continue
		}
		did, err := e.stepTokenNoting(i, order)
		schedule.Acted(id, did)
		acted = acted || did
		if err != nil {
			return acted, err
		}
	}
	for _, id := range paused {
		if e.state == StateSuspended {
			break
		}
		if i := e.tokenIndex(id); i >= 0 {
			did, err := e.stepTokenNoting(i, order)
			acted = acted || did
			if err != nil {
				return acted, err
			}
		}
	}
	return acted, nil
}

// stepInitialNode advances token from initial node to successors.
func (e *ActionExecutor) stepInitialNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	graph := e.tokenGraph(tokenIdx)
	if len(graph.Edges[token.Location]) == 0 {
		return fmt.Errorf("%w: initial node has no successors", ErrInvalidActionFlow)
	}
	if err := e.runNodeBody(token.frame, token.Location); err != nil {
		return err
	}

	successors, err := e.enabledSuccessions(token.frame, token.Location)
	if err != nil {
		return err
	}

	// A guard ruling out the succession the flow starts with ends it here.
	if len(successors) == 0 {
		return e.retireToken(tokenIdx)
	}

	// Move token to first successor (initial should have exactly 1)
	token.travel(successors[0], e.sweep)
	return nil
}

// stepFinalNode consumes token and checks for completion.
func (e *ActionExecutor) stepFinalNode(tokenIdx int) error {
	return e.retireToken(tokenIdx)
}

// removeToken drops a token from the active list without ending anything else.
func (e *ActionExecutor) removeToken(tokenIdx int) {
	e.tokens = append(e.tokens[:tokenIdx], e.tokens[tokenIdx+1:]...)
}

// retireToken ends a token's flow. Its effects live in the action's features, so
// retiring it carries nothing out; the action completes once no token is left.
// The last token of a nested flow instead leaves it, completing its node — unless
// a body statement runs that flow (the root's included), which completes the
// node once the run ends.
func (e *ActionExecutor) retireToken(tokenIdx int) error {
	frame := e.tokens[tokenIdx].frame
	if frame == e.root && !frame.inBody {
		e.removeToken(tokenIdx)
		if len(e.tokens) == 0 {
			e.state = StateCompleted
			e.ctx.endPerformanceLife(e.occurrence)
		}
		return nil
	}

	frame.live--
	if frame.live > 0 || frame.inBody {
		e.removeToken(tokenIdx)
		return nil
	}
	return e.leaveSubflow(tokenIdx)
}

// stepForkNode spawns N tokens (one per successor). A fork duplicates control
// only: its branches go on reading and writing the action's own features.
func (e *ActionExecutor) stepForkNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	node := token.Location.(*ast.ForkNode)
	frame := token.frame
	graph := e.graphOf(frame)

	if len(graph.Edges[node]) == 0 {
		return fmt.Errorf("%w: fork node %s has no successors",
			ErrInvalidActionFlow, node.Name)
	}
	if err := e.runNodeBody(frame, node); err != nil {
		return err
	}

	// A guard on a branch out of a fork prunes it: only the enabled branches run,
	// and a fork whose every branch is pruned ends the flow through it.
	successors, err := e.enabledSuccessions(frame, node)
	if err != nil {
		return err
	}
	if len(successors) == 0 {
		return e.retireToken(tokenIdx)
	}

	// Create N tokens (one per successor), in the flow the fork belongs to
	newTokens := make([]Token, 0, len(successors))
	for _, edge := range successors {
		newToken := Token{
			ID:       e.nextTokenID,
			Location: edge.Target,
			Via:      edge,
			moved:    e.sweep,
			frame:    frame,
		}
		e.nextTokenID++
		newTokens = append(newTokens, newToken)
	}

	// Remove original token, add new tokens
	e.removeToken(tokenIdx)
	e.tokens = append(e.tokens, newTokens...)
	frame.live += len(successors) - 1

	return nil
}

// stepJoinNode passes the one token a join performs with — synchronize has
// already waited for its incoming successions — on to its successor.
func (e *ActionExecutor) stepJoinNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	node := token.Location.(*ast.JoinNode)
	frame := token.frame
	graph := e.graphOf(frame)

	if err := e.runNodeBody(frame, node); err != nil {
		return err
	}

	// Get successor
	declared := graph.Edges[node]
	if len(declared) == 0 {
		return fmt.Errorf("%w: join node %s has no successors",
			ErrInvalidActionFlow, node.Name)
	}
	if len(declared) > 1 {
		return fmt.Errorf("%w: join node %s has multiple successors",
			ErrInvalidActionFlow, node.Name)
	}
	successors, err := e.enabledSuccessions(frame, node)
	if err != nil {
		return err
	}

	// A guard ruling the join's succession out ends the flow through it.
	if len(successors) == 0 {
		return e.retireToken(tokenIdx)
	}
	token.travel(successors[0], e.sweep)
	return nil
}

// stepMergeNode performs a merge for each arriving token: a merge is one MergePerformance per
// arrival (Actions::MergeAction), so a loop re-enters it and a fork's branches each traverse it.
func (e *ActionExecutor) stepMergeNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	mergeNode, ok := token.Location.(*ast.MergeNode)
	if !ok {
		return fmt.Errorf("expected MergeNode, got %T", token.Location)
	}

	graph := e.graphOf(token.frame)
	declared := graph.Edges[mergeNode]
	if len(declared) == 0 {
		return fmt.Errorf("%w: merge node %s has no successors",
			ErrInvalidActionFlow, mergeNode.Name)
	}
	if len(declared) > 1 {
		return fmt.Errorf("%w: merge node %s has multiple successors (not yet supported)",
			ErrInvalidActionFlow, mergeNode.Name)
	}
	if err := e.runNodeBody(token.frame, mergeNode); err != nil {
		return err
	}

	// The guard follows the performance (TransitionPerformances::NonStateTransitionPerformance)
	// and prunes the outgoing link, not the merge: a false guard retires the token after its body ran.
	successors, err := e.enabledSuccessions(token.frame, mergeNode)
	if err != nil {
		return err
	}
	if len(successors) == 0 {
		return e.retireToken(tokenIdx)
	}
	token.travel(successors[0], e.sweep)
	return nil
}

// stepDecisionNode evaluates guards and routes token to matching branch.
func (e *ActionExecutor) stepDecisionNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	decisionNode, ok := token.Location.(*ast.DecisionNode)
	if !ok {
		return fmt.Errorf("expected DecisionNode, got %T", token.Location)
	}

	// Get successors (outgoing edges from decision)
	graph := e.graphOf(token.frame)
	successors := graph.Edges[decisionNode]
	if len(successors) == 0 {
		return fmt.Errorf("%w: decision node %s has no successors",
			ErrInvalidActionFlow, decisionNode.Name)
	}
	if err := e.runNodeBody(token.frame, decisionNode); err != nil {
		return err
	}

	// A guard resolves in the flow's scope, with the performances' current
	// feature values pushed over it so they shadow same-named declarations.
	ec := e.evalContextFor(token.frame, graph.Scope)
	defer ec.beginStep()()

	// Two-pass evaluation:
	// 1. Evaluate all guarded edges; take the first that holds (several: a choice point)
	// 2. If none holds, use unguarded edge as fallback (else branch)

	var unguardedEdge *lower.ActionEdge
	var holding []int

	// Pass 1: Check guarded edges. Once one holds the branch is decided; the rest
	// are probed only to report the choice, which leaves the run as it was.
	for i := range successors {
		edge := &successors[i]
		// No guard = remember for fallback
		if edge.Guard == nil {
			unguardedEdge = edge
			continue
		}

		var holds bool
		if len(holding) > 0 {
			holds = e.probeGuard(token.frame, decisionNode, successors, i)
		} else {
			var err error
			if holds, err = e.guardHolds(ec, decisionNode, edge.Guard); err != nil {
				return err
			}
		}
		if holds {
			holding = append(holding, i)
		}
	}
	if len(holding) > 0 {
		choice, pick := e.chooseBranch(token.frame, decisionNode, successors, holding)
		// A refused replay move leaves the token at the decision: no branch is taken.
		if refused := e.ctx.scheduling().refusal(); refused != nil {
			return refused
		}
		// A branch picked past the first was only probed; its guard's final reading
		// is the run's own, so the run holds what evaluating it did.
		if pick > 0 {
			holds, err := e.guardHolds(ec, decisionNode, successors[holding[pick]].Guard)
			if err != nil {
				return err
			}
			if !holds {
				return fmt.Errorf("%w: decision node %s: guard of %s held when probed and not when read",
					ErrNoEnabledSuccession, decisionNode.Name, branchName(successors, holding[pick]))
			}
		}
		if choice != nil {
			e.ctx.noteChoice(*choice)
		}
		token.travel(successors[holding[pick]], e.sweep)
		return nil
	}

	// Pass 2: Use unguarded edge as fallback
	if unguardedEdge != nil {
		token.travel(*unguardedEdge, e.sweep)
		return nil
	}

	return fmt.Errorf("%w: decision node %s has no true guard",
		ErrNoEnabledSuccession, decisionNode.Name)
}

// stepActionExecutionNode evaluates inline expression or invokes nested action.
func (e *ActionExecutor) stepActionExecutionNode(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	node, ok := token.Location.(*ast.ActionExecutionNode)
	if !ok {
		return fmt.Errorf("expected ActionExecutionNode, got %T", token.Location)
	}

	frame := token.frame
	graph := frame.graph
	if node.Expression != nil {
		// Evaluate in the flow's scope, its feature values shadowing it.
		ec := e.evalContextFor(frame, graph.Scope)
		defer ec.beginStep()()
		result, err := ec.Eval(node.Expression)
		if err != nil {
			return fmt.Errorf("eval expression: %w", err)
		}

		// Store result: check if dataFlows specify output pin, else use "result"
		outputPin := "result"
		if flows, ok := graph.DataFlows[node]; ok && len(flows) > 0 {
			// Use source pin from first data flow as output pin
			if flows[0].SourcePin != "" {
				outputPin = flows[0].SourcePin
			}
		}
		if err := e.setFrameFeature(frame, outputPin, result); err != nil {
			return err
		}
	} else if node.ActionRef != nil {
		_, outputs, err := invokeAction(
			e.ctx, graph.Scope, actionInvocation{target: node.ActionRef}, lexicalValues(frame), e.self,
		)
		if err != nil {
			return err
		}
		if err := e.setFrameFeatures(frame, outputs); err != nil {
			return err
		}
	}

	// Advance to a succession its guard, where it carries one, leaves enabled.
	successors, err := e.enabledSuccessions(frame, token.Location)
	if err != nil {
		return err
	}
	if len(successors) > 1 {
		return fmt.Errorf("%w: action node %s has multiple successors (decision nodes not yet supported)",
			ErrAmbiguousSuccession, node.Name)
	}

	// Apply data flows: transfer data from this node's output pins to target input pins
	if err := e.applyDataFlows(frame, frame.graph, node, frame.data); err != nil {
		return err
	}

	if len(successors) == 0 {
		return e.retireToken(tokenIdx)
	}

	token.travel(successors[0], e.sweep)
	return nil
}

// stepNestedAction performs a nested action usage in a frame of its own.
func (e *ActionExecutor) stepNestedAction(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	usage, ok := token.Location.(*ast.Usage)
	if !ok {
		return fmt.Errorf("expected Usage, got %T", token.Location)
	}

	// An accept node waits for a message of its parameter's type. Until one
	// arrives the token parks here: the action is suspended, not failed, and
	// the next step retries the match.
	graph := e.graphOf(token.frame)
	accept, isAccept := graph.Accepts[usage]
	if isAccept && accept.Trigger != nil {
		// A trigger waits for time to pass or for a condition to hold rather
		// than for a message, so it is answered here and not from the queue.
		ready, err := e.triggerReady(token, accept)
		if ready || err != nil {
			ready, err = e.triggerHolds(token, accept)
		}
		if err != nil {
			return err
		}
		if !ready {
			if token.Wait == nil {
				token.Wait = &AcceptWait{
					ParamName: accept.ParamName,
					Trigger:   triggerDescription(accept.Trigger),
					Since:     e.stepCount + 1,
				}
			}
			return nil
		}
		token.Wait = nil
	} else if isAccept {
		// An accept node waits for the occurrence its payload names: a message of
		// the type it was typed with, or of the event it subsets.
		want := ast.QualifiedText(accept.SignalType)
		if want == "" {
			want = lower.FeaturePath(accept.SubsetsEvent)
		}
		matches, failed := e.acceptMatch(token.frame, accept, usage)
		msg, taken := e.ctx.takeAcceptable(matches)
		if *failed != nil {
			return *failed
		}
		if !taken {
			if token.Wait == nil {
				token.Wait = &AcceptWait{
					ParamName:  accept.ParamName,
					SignalType: want,
					ViaPort:    accept.ViaPort,
					// stepCount is incremented once the step finishes, so the
					// step now in progress is the next one.
					Since: e.stepCount + 1,
				}
			}
			return nil
		}
		token.Wait = nil
		if accept.ParamName != "" {
			value, err := e.ctx.acceptedValue(&msg)
			if err != nil {
				return fmt.Errorf("accept %s: %w", accept.ParamName, err)
			}
			// The payload is a feature of the flow the accept sits in, which the
			// nodes after it read by its name.
			if err := e.setFrameFeature(token.frame, accept.ParamName, value); err != nil {
				return err
			}
		}
	}

	perf, err := e.beginPerformance(token.frame, graph, usage, nil)
	if err != nil {
		return err
	}

	// A nested case is a step of its own kind: the analysis it is runs to completion.
	if isCaseStep(usage) {
		return e.runPausable(tokenIdx, func() error {
			if err := e.performCase(perf); err != nil {
				return err
			}
			return e.endPerformance(perf)
		}, func(tokenIdx int) error {
			return e.completeNode(tokenIdx, perf)
		})
	}

	// A usage that performs another action (perform X / action a : X / a = X(...))
	// runs that action to completion before its own body, pausing the token
	// while the action waits on the clock.
	if inv, ok := nestedInvocation(usage); ok {
		return e.runPausable(tokenIdx, func() error {
			return e.performInvocation(perf, inv)
		}, func(tokenIdx int) error {
			return e.performNodeBody(tokenIdx, perf, graph, usage)
		})
	}
	return e.performNodeBody(tokenIdx, perf, graph, usage)
}

// performNodeBody performs what a nested action node states of its own once the
// action it performs, if any, has completed: the flow it owns, or its statements.
func (e *ActionExecutor) performNodeBody(tokenIdx int, perf *actionFrame, graph *lower.ActionGraph, usage *ast.Usage) error {
	// A node owning a flow performs it: its steps are subperformances of the
	// node, so the node completes only once they have.
	if perf.graph != nil {
		return e.enterSubflow(tokenIdx, perf)
	}

	// Execute the node's lowered statements in declaration order.
	return e.runPausable(tokenIdx, func() error {
		if err := e.executeBody(perf, graph, usage); err != nil {
			return err
		}
		return e.endPerformance(perf)
	}, func(tokenIdx int) error {
		return e.completeNode(tokenIdx, perf)
	})
}

// completeNode takes the succession out of a node whose performance perf is
// done, retiring the token where its flow leads no further.
func (e *ActionExecutor) completeNode(tokenIdx int, perf *actionFrame) error {
	frame := e.tokens[tokenIdx].frame
	node := perf.node

	// Advance to a succession its guard, where it carries one, leaves enabled.
	successors, err := e.enabledSuccessions(frame, node)
	if err != nil {
		return err
	}
	if len(successors) > 1 {
		return fmt.Errorf("%w: action node %s has multiple successors", ErrAmbiguousSuccession, ActionNodeName(node))
	}

	// The flows out of this node carry what this performance produced to the
	// pins the nodes downstream read.
	if err := e.applyDataFlows(frame, frame.graph, node, perf.data); err != nil {
		return err
	}

	// A node the flow leads no further from is where this flow ends: the action
	// inherits its `done` snapshot, so no succession to a final node is needed.
	if len(successors) == 0 {
		return e.retireToken(tokenIdx)
	}

	e.tokens[tokenIdx].travel(successors[0], e.sweep)
	return nil
}

// triggerReady probes a change event's condition first: a test finding it not
// holding is no move and leaves no trace. A time event parks visibly, so it is not probed.
func (e *ActionExecutor) triggerReady(token *Token, accept lower.Accept) (bool, error) {
	if _, changes := accept.Trigger.(*ast.ChangeEvent); !changes {
		return true, nil
	}
	defer e.ctx.beginProbe()()
	return e.triggerHolds(token, accept)
}

// triggerHolds reports whether the time or change event an accept waits for has
// happened. A change event holds when its condition does, which every step
// re-evaluates in the action's scope with its feature values over it — the same
// polling a state machine's change transitions use. A time event parks the token
// on the context's clock and holds once the clock has reached its instant.
func (e *ActionExecutor) triggerHolds(token *Token, accept lower.Accept) (bool, error) {
	frame := token.frame
	switch t := accept.Trigger.(type) {
	case *ast.ChangeEvent:
		ec := e.evalContextFor(frame, frame.graph.Scope)
		defer ec.beginStep()()
		result, err := ec.Eval(t.Condition)
		if err != nil {
			return false, fmt.Errorf("eval accept condition: %w", err)
		}
		if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
			return false, fmt.Errorf("%w: accept when: condition must evaluate to boolean, got %v", ErrTypeMismatch, result.Kind)
		}
		return result.Const.Bool, nil
	case *ast.TimeEvent:
		if token.Wait != nil && token.Wait.Timed {
			return e.ctx.clock.now >= token.Wait.Due, nil
		}
		if err := e.ctx.judgeTimeTriggerType(frame.graph.Scope, t); err != nil {
			return false, err
		}
		ec := e.evalContextFor(frame, frame.graph.Scope)
		defer ec.beginStep()()
		val, err := ec.Eval(t.Duration)
		if err != nil {
			return false, fmt.Errorf("eval %s: %w", triggerDescription(t), err)
		}
		due, err := e.ctx.dueInstant(t, val, triggerDescription(t))
		if err != nil {
			return false, err
		}
		if due <= e.ctx.clock.now {
			return true, nil
		}
		token.Wait = &AcceptWait{
			ParamName: accept.ParamName,
			Trigger:   triggerDescription(t),
			Since:     e.stepCount + 1,
			Timed:     true,
			Due:       due,
		}
		return false, nil
	default:
		return false, fmt.Errorf("accept trigger of kind %T is not executed", accept.Trigger)
	}
}

// timeWaits lists the tokens of perf's flow (the action's for nil) parked on
// the clock, in token-ID order.
func (e *ActionExecutor) timeWaits(perf *actionFrame) []Token {
	var waiting []Token
	for _, token := range e.tokens {
		if token.Wait != nil && token.Wait.Timed && token.inFlowOf(perf) {
			waiting = append(waiting, token)
		}
	}
	sort.Slice(waiting, func(i, j int) bool { return waiting[i].ID < waiting[j].ID })
	return waiting
}

// hasDueTimeWait reports whether a token of perf's flow (the action's for nil)
// parked on the clock can proceed now.
func (e *ActionExecutor) hasDueTimeWait(perf *actionFrame) bool {
	for _, token := range e.timeWaits(perf) {
		if token.Wait.Due <= e.ctx.clock.now {
			return true
		}
	}
	return false
}

// NextWait returns the instant the earliest token parked on the clock proceeds
// at, false when none is; one already due is not a wait.
func (e *ActionExecutor) NextWait() (float64, bool) {
	waits := e.visibleWaits()
	if len(waits) == 0 {
		return 0, false
	}
	return waits[0].Due, true
}

// TimeWaits describes the tokens parked on the clock, in token-ID order, then
// those of the actions the paused work of its tokens performs.
func (e *ActionExecutor) TimeWaits() []string {
	waits := e.timeWaits(nil)
	out := make([]string, 0, len(waits))
	for _, token := range waits {
		out = append(out, token.Wait.String())
	}
	for _, held := range e.heldWaiters() {
		for _, wait := range held.clockWaits() {
			out = append(out, wait.What)
		}
	}
	return out
}

// visibleWaits lists, in due order, the waits on the clock not yet due that hold this
// action: its own, and those of the actions the paused work of its tokens performs.
func (e *ActionExecutor) visibleWaits() []ClockWait {
	return notYetDue(e.visibleArmedWaits(), e.ctx.clock.now)
}

// visibleArmedWaits lists, due or not, the waits that hold this action, earliest first:
// its own and, through the paused work of its tokens, those of the actions it performs.
func (e *ActionExecutor) visibleArmedWaits() []ClockWait {
	waits := e.armedWaits()
	for _, held := range e.heldWaiters() {
		waits = append(waits, held.visibleArmedWaits()...)
	}
	slices.SortStableFunc(waits, func(a, b ClockWait) int { return cmp.Compare(a.Due, b.Due) })
	return waits
}

// heldWaiters lists the executors performing an action for the paused work of
// this action's tokens, in token-ID order.
func (e *ActionExecutor) heldWaiters() []clockWaiter {
	tokens := slices.SortedFunc(slices.Values(e.tokens), func(a, b Token) int { return cmp.Compare(a.ID, b.ID) })
	var held []clockWaiter
	for _, token := range tokens {
		if w := token.heldWaiter(); w != nil {
			held = append(held, w)
		}
	}
	return held
}

// dueLabel names the executor in a due-order choice.
func (e *ActionExecutor) dueLabel() string {
	return actionLabelPrefix + symbolText(e.action) + performerSuffix(e.self)
}

// clockWaits lists the tokens parked on the clock for an instant it has not
// reached; an action performed for a paused body lists its own.
func (e *ActionExecutor) clockWaits() []ClockWait {
	return notYetDue(e.armedWaits(), e.ctx.clock.now)
}

// armedWaits lists the tokens parked on the clock, due or not, earliest first; an
// action performed for a paused body lists its own to the clock.
func (e *ActionExecutor) armedWaits() []ClockWait {
	var waits []ClockWait
	for _, token := range e.timeWaits(nil) {
		waits = append(waits, ClockWait{Due: token.Wait.Due, Holder: e.dueLabel(), What: token.Wait.String()})
	}
	slices.SortStableFunc(waits, func(a, b ClockWait) int { return cmp.Compare(a.Due, b.Due) })
	return waits
}

// dueWork reports a token that can move at this instant (not parked nor paused on
// the clock, due, or with a message in flight) in the flow awaiting the clock, else the action's.
func (e *ActionExecutor) dueWork() bool {
	if e.released || (e.state != StateRunning && e.state != StateWaiting) {
		return false
	}
	for _, token := range e.tokens {
		if token.Wait == nil && !token.pausedOnClock() && token.inFlowOf(e.awaiting) {
			return true
		}
	}
	return e.dueNow(e.awaiting)
}

// watchesChange reports a token parked at an `accept when`, which data written
// outside the action can let proceed.
func (e *ActionExecutor) watchesChange() bool {
	if e.released || (e.state != StateRunning && e.state != StateWaiting) {
		return false
	}
	for _, token := range e.tokens {
		if token.Wait != nil && token.Wait.Trigger != "" && !token.Wait.Timed {
			return true
		}
	}
	return false
}

// runDue runs the action to quiescence at the current instant; the steps it takes
// count against the drive's budget together with those already taken.
func (e *ActionExecutor) runDue(progress *dueProgress) (bool, error) {
	before, positions := e.stepCount, e.tokenPositions()
	e.stepsSpent = progress.steps
	err := e.run(true)
	e.stepsSpent = 0
	progress.steps += int64(e.stepCount - before)
	return e.state != StateWaiting || !maps.Equal(positions, e.tokenPositions()), err
}

// tokenPositions maps each token to where it stands, for telling a step that
// got somewhere from one that only found every token still parked.
func (e *ActionExecutor) tokenPositions() map[int64]ast.Node {
	positions := make(map[int64]ast.Node, len(e.tokens))
	for _, token := range e.tokens {
		positions[token.ID] = token.Location
	}
	return positions
}

func (e *ActionExecutor) finished() bool { return e.released || e.state == StateCompleted }
func (e *ActionExecutor) running() bool  { return e.inRun }

// performerSuffix names the object performing a behavior, nothing for none.
func performerSuffix(self *Instance) string {
	if self == nil {
		return ""
	}
	return fmt.Sprintf(" of object #%d", self.ID)
}

// stepStatementNode runs an action node member the author wrote as a statement
// and advances the token, which is what the node contributes to the flow: it
// runs the statements lowering recorded for it, then leaves for its successor.
func (e *ActionExecutor) stepStatementNode(tokenIdx int) error {
	node := e.tokens[tokenIdx].Location
	frame := e.tokens[tokenIdx].frame

	return e.runPausable(tokenIdx, func() error {
		return e.executeBody(frame, frame.graph, node)
	}, func(tokenIdx int) error {
		successors, err := e.enabledSuccessions(frame, node)
		if err != nil {
			return err
		}
		if len(successors) > 1 {
			return fmt.Errorf("%s node has multiple successors", statementNodeKeyword(node))
		}

		// As for a nested action, a node with nothing after it ends the flow.
		if len(successors) == 0 {
			return e.retireToken(tokenIdx)
		}

		e.tokens[tokenIdx].travel(successors[0], e.sweep)
		return nil
	})
}

// statementNodeKeyword names a statement node for a message about it, since a
// node written as a statement has no name to report.
func statementNodeKeyword(node ast.Node) string {
	switch n := node.(type) {
	case *ast.WhileLoopActionNode:
		return "a '" + n.Kind.String() + "' loop"
	case *ast.IfActionNode:
		return "an 'if'"
	case *ast.AssignmentActionNode:
		return "an 'assign'"
	case *ast.SendStatement:
		return "a 'send'"
	case *ast.TerminateStatement:
		return "a 'terminate'"
	default:
		return fmt.Sprintf("a %T", node)
	}
}

// applyDataFlows moves what the completed performance produced along graph's flows out
// of sourceNode to the target pins; a source pin holding nothing is an error, not a no-op.
func (e *performances) applyDataFlows(frame *actionFrame, graph *lower.ActionGraph, sourceNode ast.Node, produced map[string]Value) error {
	for _, flow := range graph.DataFlows[sourceNode] {
		sourceData, ok := produced[flow.SourcePin]
		if !ok {
			return fmt.Errorf(
				"%s: %s produced no value at %s",
				flowDescription(flow), nodeDescription(sourceNode), orAnyPin(flow.SourcePin),
			)
		}
		if err := e.deliverFlow(frame, graph, flow, sourceData); err != nil {
			return err
		}
	}
	return nil
}

// deliverFlow puts a flow's payload where its target reads it: at the pin of a
// target performing in a frame of its own, else in the flow's own features.
func (e *performances) deliverFlow(frame *actionFrame, graph *lower.ActionGraph, flow lower.ObjectFlow, value Value) error {
	if _, performs := flow.Target.(*ast.Usage); performs {
		if err := e.deliver(frame, graph, flow.Target, nil, flow.TargetPin, value); err != nil {
			return fmt.Errorf("%s: %w", flowDescription(flow), err)
		}
		return nil
	}
	return e.setFrameFeature(frame, flow.TargetPin, value)
}

// flowDescription names a data flow for a diagnostic: its own name when it was
// declared with one, and the pins it joins otherwise.
func flowDescription(flow lower.ObjectFlow) string {
	if flow.Name != "" {
		return "flow " + flow.Name
	}
	return fmt.Sprintf(
		"flow from %s to %s",
		orAnyPin(flow.SourcePin), orAnyPin(flow.TargetPin),
	)
}

// orAnyPin names a pin an end left implicit.
func orAnyPin(pin string) string {
	if pin == "" {
		return "its output"
	}
	return pin
}

// nodeDescription names an action node for a diagnostic, falling back to the
// kind of node it is when the notation gave it no name.
func nodeDescription(node ast.Node) string {
	if name := ActionNodeName(node); name != "" {
		return "node " + name
	}
	return fmt.Sprintf("the %T", node)
}

// --- Public accessor methods for REPL debugging ---

// Tokens returns a copy of active tokens.
func (e *ActionExecutor) Tokens() []Token {
	tokens := make([]Token, len(e.tokens))
	copy(tokens, e.tokens)
	return tokens
}

// State returns current execution state.
func (e *ActionExecutor) State() ExecutionState {
	return e.state
}

// Results returns the values the action's features hold, and under `node.pin` those
// of each nested node's latest performance; a performed usage's mirror its occurrence.
func (e *ActionExecutor) Results() map[string]Value {
	results := make(map[string]Value, len(e.root.data))
	e.root.collect("", results)
	return results
}

// Data returns the live feature space of the action's own performance; a nested
// node's features live in its own performance and are reported by Results.
func (e *ActionExecutor) Data() map[string]Value {
	return e.root.data
}

// Held is a copy of what a performance holds at one moment: the features it
// holds, and their values looked up under the names the performance keys them by.
type Held struct {
	features []lower.Attribute
	data     map[string]Value
	aliases  map[string]string
}

// Held copies what the action's own performance holds now.
func (e *ActionExecutor) Held() Held {
	return Held{
		features: slices.Clone(e.features),
		data:     maps.Clone(e.root.data),
		aliases:  maps.Clone(e.root.aliases),
	}
}

// Features are the attributes and parameters the performance holds, the graph's
// own then the inherited ones it does not redefine, as the run initializes them.
func (h Held) Features() []lower.Attribute {
	return h.features
}

// Value is the value held under name: its redefinition's when name is redefined.
func (h Held) Value(name string) (Value, bool) {
	v, ok := h.data[canonical(h.aliases, name)]
	return v, ok
}

// SetBreakpoint adds a breakpoint at the given node name.
func (e *ActionExecutor) SetBreakpoint(nodeName string) {
	e.breakpoints[nodeName] = true
}

// ClearBreakpoints removes all breakpoints.
func (e *ActionExecutor) ClearBreakpoints() {
	e.breakpoints = make(map[string]bool)
	e.firedBreakpoints = make(map[breakpointVisit]bool)
}

// trace returns the recorder this executor's context is attached to, so turning
// reporting on or off reaches an execution already under way.
func (e *ActionExecutor) trace() *TraceRecorder {
	return e.ctx.trace
}

// SetTrace sets the trace recorder for this executor and the context it
// evaluates in.
func (e *ActionExecutor) SetTrace(trace *TraceRecorder) {
	e.ctx.SetTrace(trace)
}

// ActionSymbol returns the action being executed.
func (e *ActionExecutor) ActionSymbol() *symbols.Symbol {
	return e.action
}

// Graph is the lowered flow the run performs, the one every step of it consumes.
func (e *ActionExecutor) Graph() *lower.ActionGraph {
	return e.graph
}

// Performer returns the object performing the action, nil for an action
// performed outside any object.
func (e *ActionExecutor) Performer() *Instance {
	return e.self
}
