package runtime

import (
	"errors"
	"fmt"
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

// ActionExecutor executes action bodies using token-flow semantics.
type ActionExecutor struct {
	// performances holds the action's own performance, root, and runs its nodes' as
	// its subperformances; self is the object performing the action, whose
	// connections route what it sends.
	performances
	action *symbols.Symbol
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
	inputs           map[string]Value // Input parameter bindings, applied over attribute defaults
	pausedAt         string           // Node name RunToCompletion stopped at, empty when it ran to the end
	// pause is set while a token's work runs as a coroutine (action_body_run.go):
	// called with a breakpoint's name, it pauses the run there until resumed.
	pause func(string) bool
	// released is set once Release has ended the run for good.
	released bool
	// pauses counts the body pauses so far, ordering the paused runs' resumption.
	pauses int64

	// runStarted marks this executor's run as begun, so the step budget is reset
	// once however many calls the run is driven over.
	runStarted bool
	// steps of the action's token-flow budget the current call has spent, by the
	// tokens of its flow and of the flows body statements run alike.
	steps int64
	// inRun is set while RunToCompletion drives the steps, whose budget they share.
	inRun bool
}

// chargeActionStep spends one step of the action's token-flow budget
// (MaxActionStepsEnvVar), which the tokens of every flow of the run share.
func (e *ActionExecutor) chargeActionStep() error {
	if e.steps >= e.ctx.maxActionSteps {
		return budgetExceeded(ErrActionStepLimitExceeded,
			fmt.Sprintf("execution exceeded max steps (%d steps; raise %s to allow more), possible infinite loop",
				e.ctx.maxActionSteps, MaxActionStepsEnvVar))
	}
	e.steps++
	return nil
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
	return newActionExecutorForOccurrence(ctx, action, self, nil)
}

// newActionExecutorForOccurrence creates an executor whose action's own features
// are held by the given performance occurrence, nil for an action performed
// through no usage of an object.
func newActionExecutorForOccurrence(
	ctx *Context,
	action *symbols.Symbol,
	self *Instance,
	occurrence *Instance,
) (*ActionExecutor, error) {
	if action.Kind != symbols.SymbolActionUsage && action.Kind != symbols.SymbolActionDef {
		return nil, fmt.Errorf("symbol %s is not an action", action.Name)
	}
	if err := ctx.checkPerformer(self); err != nil {
		return nil, err
	}

	// A usage stating no body of its own performs the body of the action it
	// names — the definition typing it — as a classifier behavior binding does.
	action = ctx.actionBodySymbol(action)

	// Lower AST to execution graph, in the scope the action's body was written
	// in, so that everything the graph carries is evaluated where it was declared.
	graph, err := lower.ToActionGraph(action.Decl, declScope(action))
	if err != nil {
		return nil, fmt.Errorf("lower action graph: %w", err)
	}

	exec := &ActionExecutor{
		performances: performances{ctx: ctx, self: self},
		action:       action,
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

	return exec, nil
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
	for _, member := range e.ctx.model.MembersOf(e.action) {
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
				features[i].Value, features[i].Scope = e.ctx.model.ParameterDefault(member)
			}
			continue
		}
		declared[name] = len(features)
		value, scope := e.ctx.model.ParameterDefault(member)
		features = append(features, lower.Attribute{Name: name, Value: value, Node: usage, Scope: scope})
	}
	return features
}

// Step advances execution by one step for all active tokens.
// Safely handles token slice modifications (fork/join) by collecting indices first.
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
	defer e.ctx.beginExecutorRun(&e.runStarted)()

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

	// Collect token indices to step (snapshot before iteration)
	tokenIndices := make([]int, len(e.tokens))
	for i := range e.tokens {
		tokenIndices[i] = i
	}

	// The tokens a breakpoint left paused resume last, the longest paused first,
	// once every other token has had its step: one pausing again and again does
	// not hold the rest back.
	paused := e.pausedTokens()

	// Step tokens in reverse order to handle removal safely
	// (removing token at higher index doesn't affect lower indices)
	for i := len(tokenIndices) - 1; i >= 0 && e.state != StateSuspended; i-- {
		// Check if token still exists (may have been removed by join/final)
		if i >= len(e.tokens) || e.tokens[i].drivenByBody() || e.tokens[i].body != nil {
			continue
		}

		if err := e.stepToken(i); err != nil {
			e.endPausedBodies()
			return err
		}
	}
	for _, id := range paused {
		if e.state == StateSuspended {
			break
		}
		if i := e.tokenIndex(id); i >= 0 {
			if err := e.stepToken(i); err != nil {
				e.endPausedBodies()
				return err
			}
		}
	}

	// A step a breakpoint ends leaves every other token where it was, yet the run
	// went on.
	progressMade := e.state == StateSuspended

	// Progress indicators:
	// 1. Token count changed (fork/join/final consumed/created tokens)
	if len(e.tokens) != tokenCountBefore {
		progressMade = true
	}

	// 2. At least one token moved to different location
	if !progressMade && len(e.tokens) > 0 {
		for i := 0; i < len(e.tokens) && i < len(tokenLocationsBefore); i++ {
			if e.tokens[i].Location != tokenLocationsBefore[i] {
				progressMade = true
				break
			}
		}
	}

	// 3. All tokens consumed (completion)
	if len(e.tokens) == 0 {
		progressMade = true
	}

	// If no progress and tokens remain, either the action is suspended waiting
	// for a message, or it is stuck for a reason no message can resolve.
	if !progressMade && len(e.tokens) > 0 {
		if !e.anyTokenWaiting() {
			return fmt.Errorf("%w: %d token(s) stuck, no progress made",
				ErrActionDeadlock, len(e.tokens))
		}
		e.state = StateWaiting
	}

	// Increment step count
	e.stepCount++

	// Record trace after step completes
	if e.trace() != nil {
		e.trace().RecordActionStep(e.stepCount, e.tokens)
	}

	return nil
}

// anyTokenWaiting reports whether some token is parked at an accept.
func (e *ActionExecutor) anyTokenWaiting() bool {
	for _, token := range e.tokens {
		if token.Wait != nil {
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
	waiting := e.waitingTokens(perf)
	descriptions := make([]string, 0, len(waiting))
	for _, token := range waiting {
		descriptions = append(descriptions, token.Wait.String())
	}
	if blocked := len(e.tokensIn(perf)) - len(waiting); blocked > 0 {
		descriptions = append(descriptions,
			fmt.Sprintf("%d token(s) blocked for another reason", blocked))
	}
	where := "action " + e.action.Name
	if perf != nil {
		where = perf.describe()
	}
	return fmt.Errorf("%w in %s: nothing can post the awaited message (%s)",
		ErrAcceptDeadlock, where, strings.Join(descriptions, "; "))
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
// whose every remaining token is parked at an accept can never be resumed: the
// suspension is a deadlock and is reported as ErrAcceptDeadlock at the first
// step that makes no progress. A parked action therefore cannot spend the step
// budget spinning — the budget is only consumed by steps that move something.
func (e *ActionExecutor) RunToCompletion() error {
	defer e.ctx.beginExecutorRun(&e.runStarted)()

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
	for e.state == StateRunning || e.state == StateWaiting {
		if node := e.breakpointHit(); node != "" {
			e.pausedAt = node
			e.state = StateSuspended
			return nil
		}

		if err := e.chargeActionStep(); err != nil {
			e.endPausedBodies()
			return err
		}

		if err := e.Step(); err != nil {
			return err
		}
		if e.state == StateWaiting {
			e.endPausedBodies()
			return e.deadlockError(nil)
		}
	}

	return nil
}

// RunToQuiescence runs the action until it completes, stops at a breakpoint, or
// parks every remaining token at an accept. Unlike RunToCompletion, a parked
// action is quiescence rather than a deadlock: this is what an object performing
// an action is run with, where a sibling object may still send the awaited
// message.
func (e *ActionExecutor) RunToQuiescence() error {
	if err := e.RunToCompletion(); err != nil && !errors.Is(err, ErrAcceptDeadlock) {
		return err
	}
	return nil
}

// HasPendingSignal reports whether a message in flight would let a parked token
// proceed, without consuming it.
func (e *ActionExecutor) HasPendingSignal() bool {
	for _, token := range e.tokens {
		if token.Wait == nil {
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
		matches, failed := e.acceptMatch(token.frame, accept, usage)
		for _, msg := range e.ctx.PendingMessages() {
			// A port that fails to resolve counts as pending: the step this
			// provokes surfaces the failure.
			if matches(msg) || *failed != nil {
				return true
			}
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

// breakpointHit returns the name of a breakpoint node a token sits on and has
// not yet stopped the run at, or "" if none does. Firing once per token and
// visit means a resumed run continues past the node it stopped at, while a
// token that leaves and comes back around a loop stops again.
func (e *ActionExecutor) breakpointHit() string {
	if len(e.breakpoints) == 0 {
		return ""
	}
	for visit := range e.firedBreakpoints {
		if loc, ok := e.tokenLocation(visit.token); !ok || loc != visit.node {
			delete(e.firedBreakpoints, visit)
		}
	}
	for _, token := range e.tokens {
		name := e.breakpointNameOf(token.Location)
		if name == "" {
			continue
		}
		visit := breakpointVisit{token: token.ID, node: token.Location}
		if e.firedBreakpoints[visit] {
			continue
		}
		if e.firedBreakpoints == nil {
			e.firedBreakpoints = make(map[breakpointVisit]bool)
		}
		e.firedBreakpoints[visit] = true
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
	} else if err := e.ctx.checkNamedWrite(e.graph.Scope, "action "+symbolText(e.action), name, value); err != nil {
		// No occurrence holds this feature, so its declaration is checked here
		// rather than by the write to that occurrence.
		return err
	}
	e.root.data[e.root.key(name)] = value
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
// initial node has no step to perform.
func (e *ActionExecutor) hasFlow() bool {
	return e.graph != nil && e.graph.Initial != nil
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
	defer e.ctx.beginExecutorRun(&e.runStarted)()

	// Use initial node from graph
	if e.graph.Initial == nil {
		return fmt.Errorf("%w: no initial node found in action %s",
			ErrInvalidActionFlow, e.action.Name)
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
	if token.Via == (lower.ActionEdge{}) || !synchronizes(token.Location) {
		return tokenIdx, true
	}
	_, join := token.Location.(*ast.JoinNode)
	incoming := e.awaitedSuccessions(token.frame, token.Location)
	if len(incoming) < 2 && !join {
		return tokenIdx, true
	}

	consumed := make([]int, 0, len(incoming))
	for _, edge := range incoming {
		idx, ok := e.arrival(token.frame, token.Location, edge)
		if !ok {
			return tokenIdx, false
		}
		consumed = append(consumed, idx)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(consumed)))
	for _, idx := range consumed {
		e.removeToken(idx)
	}
	token.frame.live -= len(consumed) - 1
	e.tokens = append(e.tokens, Token{ID: e.nextTokenID, Location: token.Location, frame: token.frame})
	e.nextTokenID++
	return len(e.tokens) - 1, true
}

// synchronizes reports whether a node waits for all its incoming successions: every node
// does but a merge, the one multi-incoming node that fires per arrival (Actions::MergeAction).
func synchronizes(node ast.Node) bool {
	_, merge := node.(*ast.MergeNode)
	return !merge
}

// arrival returns the earliest token of frame held at node that arrived over edge.
func (e *ActionExecutor) arrival(frame *actionFrame, node ast.Node, edge lower.ActionEdge) (int, bool) {
	for i, t := range e.tokens {
		if t.Location == node && t.frame == frame && t.body == nil && t.Via == edge {
			return i, true
		}
	}
	return 0, false
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
		if _, delivered := e.arrival(frame, node, edge); delivered || live[edge.Source] {
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
		if _, delivered := e.arrival(frame, node, edge); !delivered && !reached[edge.Source] {
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
		if _, delivered := e.arrival(token.frame, token.Location, edge); !delivered {
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
	if guard == nil {
		return true, nil
	}
	result, err := ec.Eval(guard)
	if err != nil {
		return false, fmt.Errorf("eval guard of %s: %w", nodeDescription(node), err)
	}
	if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("%w: %s: guard must evaluate to boolean, got %v",
			ErrTypeMismatch, nodeDescription(node), result.Kind)
	}
	return result.Const.Bool, nil
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
	token.travel(successors[0])
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
	token.travel(successors[0])
	return nil
}

// stepMergeNode passes each arriving token on: a merge is one MergePerformance per
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
	successors, err := e.enabledSuccessions(token.frame, mergeNode)
	if err != nil {
		return err
	}
	if len(successors) == 0 {
		return e.retireToken(tokenIdx)
	}

	// The body runs with the traversal, not with an arrival whose succession was pruned.
	if err := e.runNodeBody(token.frame, mergeNode); err != nil {
		return err
	}
	token.travel(successors[0])
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
	// 1. Evaluate all guarded edges first
	// 2. If none match, use unguarded edge as fallback (else branch)

	var unguardedEdge *lower.ActionEdge

	// Pass 1: Check guarded edges
	for i := range successors {
		edge := &successors[i]
		// No guard = remember for fallback
		if edge.Guard == nil {
			unguardedEdge = edge
			continue
		}

		holds, err := e.guardHolds(ec, decisionNode, edge.Guard)
		if err != nil {
			return err
		}
		if holds {
			token.travel(*edge)
			return nil
		}
	}

	// Pass 2: Use unguarded edge as fallback
	if unguardedEdge != nil {
		token.travel(*unguardedEdge)
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

	token.travel(successors[0])
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
		ready, err := e.triggerHolds(token.frame, accept)
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
		msg, taken := e.ctx.TakeMessage(matches)
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
	// runs that action to completion before its own body.
	if inv, ok := nestedInvocation(usage); ok {
		if err := e.performInvocation(perf, inv); err != nil {
			return err
		}
	}

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

	e.tokens[tokenIdx].travel(successors[0])
	return nil
}

// triggerHolds reports whether the time or change event an accept waits for has
// happened. A change event holds when its condition does, which every step
// re-evaluates in the action's scope with its feature values over it — the same
// polling a state machine's change transitions use. A time event needs a clock
// the action executor does not have, so it is reported rather than treated as
// having already fired.
func (e *ActionExecutor) triggerHolds(frame *actionFrame, accept lower.Accept) (bool, error) {
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
		return false, fmt.Errorf("%w: %s in an action body: a time event is only waited on by a state machine's transitions",
			ErrNoClock, triggerDescription(t))
	default:
		return false, fmt.Errorf("accept trigger of kind %T is not executed", accept.Trigger)
	}
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

		e.tokens[tokenIdx].travel(successors[0])
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
