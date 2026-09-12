package runtime

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Context is the run-derived state of one execution over a Model: the objects
// it created, their values and lifetimes, the messages in flight, the clock, the
// executors' progress. A snapshot captures it; the Model it runs over is shared
// with every other run of the same model and is not part of it.
type Context struct {
	// model is the model-derived part every context over one model shares.
	model *Model
	// ids hands out instance identities. Contexts holding the same objects share
	// one sequence, so no two of them name different objects alike.
	ids *idSequence
	// took is one past the highest identity this context took from ids (see idMark).
	took      *idMark
	maxSteps  int64
	instances map[int64]*Instance
	created   []int64
	// lives holds, per registered object, when it began and ended (lifetimes.go).
	lives map[int64]life

	// maxActionSteps, maxStateEvents and maxDoSteps bound the executors this
	// context runs: token-flow steps, dispatched events, and do actions.
	// Unlike maxSteps they are counted by the executor, not here.
	maxActionSteps int64
	maxStateEvents int64
	maxDoSteps     int64

	// maxElements bounds the collection elements one run materializes, which is
	// what its memory grows with, unlike a step.
	maxElements int64

	// framesReading holds the frame each object being read is (nil for a
	// transformation), so `target = that` finds it and a cycle is reported.
	framesReading map[int64]*CoordinateFrame

	// evaluations is the log of the case run under way (evaluation_log.go), nil
	// outside one.
	evaluations *evaluationLog

	// calcUsageRunning holds the calc usages whose bodies are running, so a body
	// reading its own usage is a recursion rather than a nested evaluation.
	calcUsageRunning map[calcUsageKey]*calcShape

	// activations numbers the body activations begun in this context: a calc
	// invocation, a block entry, a loop iteration, a body application.
	activations int64
	// runs numbers the behavior runs begun in this context — calc invocations, calc
	// usage evaluations, action performances — which functions closing over one carry.
	runs int64

	// occurrences holds the object each usage carrying no value of its own
	// denotes, so a feature chain through a part reads one occurrence of it.
	occurrences map[*symbols.Symbol]int64
	// namespaceBindings holds the value each namespace-level object usage given a value
	// denotes, so every read of it reads the one binding rather than evaluating it anew.
	namespaceBindings map[*symbols.Symbol]Value
	// bindingStack holds the namespace-level usages whose values are being evaluated, innermost
	// last, so a value reaching back to its own usage is a cycle, and an extent finds no object of it yet.
	bindingStack []*symbols.Symbol
	// bindingReads holds, per bound usage, the declarations its value read to arrive at the binding,
	// so a re-analysis carries the binding only while every one of them still reads the same.
	bindingReads map[*symbols.Symbol]*bindingReads
	// metadataObjects holds the object each metadata annotation denotes, so
	// reading `.metadata` twice reads one object per annotation. The annotation
	// is named by the element it annotates and its place among that element's
	// annotations, so a reanalysis can rebind it.
	metadataObjects map[metadataAnnotation]int64
	// tools runs the external tool a ToolExecution names; nil refuses every such action.
	tools ToolRunner

	// variantObjects holds the object a variant stands for per owner that
	// selected it, so repeated reads of one selection read the same object.
	variantObjects map[variantObject]int64

	// selectedVariants records, per owner and variation name, the variant bound
	// to it in this run. Routing consults it: a connection a `variant interface`
	// declares joins its ends only where that variant is the one selected.
	selectedVariants map[variantSelection]string

	// materializingConnectors holds the connectors whose ends are being attached,
	// so a connector reached from its own end is reported as a cycle.
	materializingConnectors map[connectorRef]bool

	// resolvingBindings guards binding endpoint resolution for one instance
	// feature, so a valueless binding cycle is reported rather than recursed.
	resolvingBindings map[featureValueRef]bool
	bindingOwners     map[featureValueRef]*ast.Usage

	// pendingBehaviors are the object behaviors attached but not yet run, drained
	// by the outermost materialization so a start reached from inside a running
	// behavior does not run it recursively.
	pendingBehaviors []*ObjectBehavior

	// behaviorRunDepth is the number of classifier-behavior starts under way.
	behaviorRunDepth int

	// declarative makes the context read declared values only: no classifier
	// behavior starts when an object is materialized (see DeclaredReader).
	declarative bool

	// heldBehaviors are the behaviors already holding work when the outermost
	// start under way began: a driver put it in flight, and dispatches it.
	heldBehaviors map[*ObjectBehavior]bool

	// objectBehaviors are every behavior an object of this context runs, so a
	// drain to quiescence can re-run one a sibling's send woke.
	objectBehaviors []*ObjectBehavior

	// trace records evaluation, nil when not tracing.
	trace *TraceRecorder
	// stepWrites is the ledger of the action step under way, nil between steps.
	stepWrites *stepWriteLedger

	// actionDepth is the number of action invocations currently on the stack,
	// bounding recursion across nested action executors.
	actionDepth int

	// pausable is the body run on the stack a breakpoint or a wait on the clock
	// pauses (action_body_run.go), nil while none is.
	pausable *bodyRun
	// clockHeldBy names the behavior on the stack that must end at the instant it
	// runs at, so no wait on the clock under it may advance the clock; "" for none.
	clockHeldBy string
	// idleBody is the body coroutine no step's work is on, kept for the next step
	// until the outermost run leaves; bodyCoroutinesMade counts the ones made.
	idleBody           *bodyCoroutine
	bodyCoroutinesMade int

	// calcDepth is the number of calc invocations currently on the stack, which
	// maxCalcDepth bounds, so a recursion evaluates while it stays within it.
	calcDepth    int
	maxCalcDepth int64

	// maxSweepRuns bounds the runs one parameter sweep or sample asks for.
	maxSweepRuns int64

	// freeInvocationFrames are the frames of returned calc invocations, kept so a
	// recursion reuses storage rather than allocating per call.
	freeInvocationFrames []*invocationFrame

	// argStack holds the positional arguments of the calc invocations under way,
	// innermost last, so an invocation borrows rather than allocates its storage.
	argStack []Value

	// scalarStack holds the frames of the compiled calc invocations under way —
	// parameters, then body locals — innermost last, the compiled tier's
	// counterpart of argStack.
	scalarStack []scalar

	// libraryArgBuf holds the boxed arguments of the library call a compiled body
	// is making, and libraryEval the context a collection built-in it calls takes.
	libraryArgBuf []Value
	libraryEval   EvalContext

	// compileCalcs enables the compiled tier for eligible calc bodies; the
	// OPENSYSML_CALC_COMPILE escape hatch clears it.
	compileCalcs bool

	// probes is the number of probes under way; see beginProbe.
	probes int
	// journals is the number of probes and transactions under way: while one is,
	// every change is journaled for it to undo; see beginJournal.
	journals int
	// journalWrites are the feature values the journals under way changed, with
	// what each held before, restored as each is undone.
	journalWrites []journalWrite
	// journalUndos restore what else the journals under way changed on an object —
	// the identities it keeps for connectors not yet materialized — run in reverse
	// as each is undone; see noteProbeUndo.
	journalUndos []func()
	// snapshots are the live snapshots of this context, oldest first (snapshot.go);
	// each is a journal under way.
	snapshots []*Snapshot
	// deriving are the `=` values being derived, innermost last; every feature
	// value read while one is records it as a dependent (see dependents.go).
	deriving []derivation
	// runBoundaries mark, innermost last, where in objectBehaviors and in
	// pendingBehaviors the behaviors a change still to be kept or undone attached
	// begin: the only ones a drain under it may run (see nextRunnableBehavior).
	runBoundaries []runBoundary
	// run is the state of the run under way, or of the latest one ended; see beginRun.
	run *runState
	// runDepth is the number of runs currently under way, so the state is installed
	// per top-level run rather than kept over the context's whole life.
	runDepth int

	// schedule is the policy the next run resolves its choice points under.
	schedule SchedulePolicy
	// exploring is the exploration run this context's runs take part in, nil
	// outside Explore (explore.go).
	exploring *exploreRun
	// replaying is the witness the context's runs follow in turn since a `replay`
	// policy was set, nil under any other (replay.go).
	replaying *replayRun
	// choices are the choice points the context's runs resolved, in order: the
	// witness a replay of them follows.
	choices []ChoiceTaken

	// messages are the signals in flight, oldest first. The bus is context-wide,
	// so a message one behavior sends can be accepted in another.
	messages []Message
	// mail, while a state's do behavior runs, is where its accepts look in place
	// of the bus: the message its machine dispatched to it, none between dispatches.
	mail *[]Message

	// clock is the simulation time every executor of this context shares, and
	// clockRun the run an advance of it draws its due-order choices from.
	clock    Clock
	clockRun executorRun

	// derivingFeatureValues holds the feature values whose defaults are being evaluated, so a
	// default that refers back to its own feature value is reported as a cycle.
	derivingFeatureValues map[featureValueRef]bool

	// collectingSubsets holds the feature values whose subsetting features are being read,
	// so features that subset each other are reported as a cycle.
	collectingSubsets map[featureValueRef]bool
	// readingSubsetted holds the optional features whose subsetted collections are
	// being read ahead of them, so two subsetting each other do not recurse.
	readingSubsetted map[featureValueRef]bool
}

// featureValueRef identifies one feature value of one instance.
type featureValueRef struct {
	instance int64
	feature  string
}

// featureOfType names a declared feature read as a feature of one type.
type featureOfType struct {
	feature, owner *symbols.Symbol
}

// variantSelection identifies a variation point of one object: two objects of a
// type each select their own variant of the same variation.
type variantSelection struct {
	owner     int64
	variation string
}

// connectorRef identifies one connector being materialized in the context of
// the object whose features its ends name.
type connectorRef struct {
	owner     int64
	connector *symbols.Symbol
}

// NewContext creates a run's context over the model-derived part model.
// maxSteps sets the runaway guard (step counter limit); the executor bounds take
// their defaults, which SetBudgets replaces.
// It panics if maxSteps <= 0: the limit is a programmer-supplied invariant, not
// user input, so callers must pass a positive value.
func NewContext(model *Model, maxSteps int64) *Context {
	if maxSteps <= 0 {
		panic(fmt.Sprintf("runtime: maxSteps must be > 0, got %d", maxSteps))
	}
	if model == nil {
		panic("runtime: NewContext needs a Model")
	}
	ctx := &Context{
		model:     model,
		maxSteps:  maxSteps,
		instances: make(map[int64]*Instance),
		lives:     make(map[int64]life),

		compileCalcs: CalcCompileFromEnv(),

		run:              &runState{calcUsageRuns: make(map[int64]map[calcUsageKey]*calcRun)},
		calcUsageRunning: make(map[calcUsageKey]*calcShape),

		maxActionSteps: DefaultMaxActionSteps,
		maxStateEvents: DefaultMaxStateEvents,
		maxDoSteps:     DefaultMaxDoSteps,
		maxElements:    DefaultMaxElements,
		maxCalcDepth:   DefaultMaxCalcDepth,
		maxSweepRuns:   DefaultMaxSweepRuns,

		occurrences:       make(map[*symbols.Symbol]int64),
		namespaceBindings: make(map[*symbols.Symbol]Value),
		bindingReads:      make(map[*symbols.Symbol]*bindingReads),
		metadataObjects:   make(map[metadataAnnotation]int64),
		variantObjects:    make(map[variantObject]int64),
		selectedVariants:  make(map[variantSelection]string),

		materializingConnectors: make(map[connectorRef]bool),
		derivingFeatureValues:   make(map[featureValueRef]bool),
		resolvingBindings:       make(map[featureValueRef]bool),
		bindingOwners:           make(map[featureValueRef]*ast.Usage),
		collectingSubsets:       make(map[featureValueRef]bool),
		readingSubsetted:        make(map[featureValueRef]bool),
	}
	ctx.took = &idMark{high: 1}
	ctx.ids = newIDSequence(ctx.took)
	return ctx
}

// Model returns the model-derived part this context runs over.
func (ctx *Context) Model() *Model {
	return ctx.model
}

// declaredSymbol is the symbol a scope tree registered with the model declares
// for the declaration sym stands for, or sym itself when none does.
func (ctx *Context) declaredSymbol(sym *symbols.Symbol) *symbols.Symbol {
	return ctx.model.declaredSymbol(sym)
}

// collectDeclared records the symbol declared by each node under scope.
func collectDeclared(scope *symbols.Scope, into map[ast.Node]*symbols.Symbol) {
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if sym.Decl != nil {
			into[sym.Decl] = sym
		}
		return true
	})
	for _, child := range scope.Children() {
		collectDeclared(child, into)
	}
}

// sourceLocation renders where a span in a file was written, as
// `file:line:col`. It falls back to a byte offset for a file whose text was not
// registered, and to the file name alone when there is no span, so a diagnostic
// always says as much as the context knows.
func (ctx *Context) sourceLocation(file string, span source.Span) string {
	if file == "" {
		return ""
	}
	sf, ok := ctx.model.sources[file]
	if !ok || span.End() > sf.Len() {
		if span.Len == 0 && span.Offset == 0 {
			return file
		}
		return fmt.Sprintf("%s:#%d", file, span.Offset)
	}
	pos := sf.Lines().PosAt(span.Offset)
	return fmt.Sprintf("%s:%d:%d", file, pos.Line, pos.Col)
}

// symbolLocation renders where a symbol was declared, empty for none.
func (ctx *Context) symbolLocation(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	return ctx.sourceLocation(sym.DocName, sym.DeclSpan)
}

// SetTrace attaches a trace recorder to this context, so that every expression
// and calc evaluated through it is recorded. Pass nil to stop tracing.
func (ctx *Context) SetTrace(tr *TraceRecorder) {
	ctx.trace = tr
}

// Trace is the recorder attached to this context, nil when not tracing.
func (ctx *Context) Trace() *TraceRecorder {
	return ctx.trace
}

// SetSchedule sets the policy the runs started from now on resolve their choice
// points under; a run already under way keeps the one it started with. An
// `explore` policy is ErrExploreUndriven: it is driven by Explore. A `replay`
// policy starts its witness over, which the runs then follow in turn.
func (ctx *Context) SetSchedule(policy SchedulePolicy) error {
	if _, explores := policy.Exploration(); explores {
		return fmt.Errorf("%w: %s replays whole runs from the start, so it is driven by Explore", ErrExploreUndriven, policy)
	}
	ctx.schedule = policy
	ctx.replaying = nil
	if policy.kind == scheduleReplay {
		ctx.replaying = &replayRun{choices: policy.replay.choices}
	}
	return nil
}

// beginExploration makes the context's runs the given run of an exploration; the
// state installed for a run no bracket began starts over, drawing from it.
func (ctx *Context) beginExploration(policy SchedulePolicy, run *exploreRun) {
	ctx.schedule = policy
	ctx.exploring, ctx.replaying = run, nil
	ctx.run = ctx.newRunState()
}

// newScheduler starts the resolutions of one run under the context's policy.
func (ctx *Context) newScheduler() *scheduler {
	s := ctx.schedule.start()
	s.explore = ctx.exploring
	s.replay = ctx.replaying
	return s
}

// Schedule returns the policy the next run resolves its choice points under.
func (ctx *Context) Schedule() SchedulePolicy {
	return ctx.schedule
}

// scheduling returns the resolutions the run under way draws, starting them for
// a run no bracket began.
func (ctx *Context) scheduling() *scheduler {
	if ctx.run.scheduler == nil {
		ctx.run.scheduler = ctx.newScheduler()
	}
	return ctx.run.scheduler
}

// Semantics returns the semantic model this context operates over.
func (ctx *Context) Semantics() *semantics.Model {
	return ctx.model.semantics
}

// conforms is the model's conformance across scope trees: the index and a
// document each build a symbol of their own for one declaration, so a symbol
// conforms to another declared by the same node as it or one of its supertypes.
func (ctx *Context) conforms(a, b *symbols.Symbol) bool {
	if ctx.modelConforms(a, b) {
		return true
	}
	if a == nil || b == nil || b.Decl == nil {
		return false
	}
	if a.Decl == b.Decl {
		return true
	}
	for _, sup := range ctx.model.semantics.AllSupertypes(a) {
		if sup != nil && sup.Decl == b.Decl {
			return true
		}
	}
	return false
}

// Resolver returns the name resolver this context resolves references with.
func (ctx *Context) Resolver() *resolve.Resolver {
	return ctx.model.resolver
}

// SourceLocation renders where a span in a file was written, as `file:line:col`,
// falling back to a byte offset for a file whose text was not registered.
func (ctx *Context) SourceLocation(file string, span source.Span) string {
	return ctx.sourceLocation(file, span)
}

// idSequence hands out instance identities, one per object over the contexts
// sharing it. Of each it keeps only its mark, so a context dropped is not kept alive.
type idSequence struct {
	next  int64
	marks []*idMark
}

// idMark is one past the highest identity a context took from its sequence and
// has not rolled back; it outlives the context, so what a dropped one took stays taken.
type idMark struct {
	high int64
}

// newIDSequence starts the identities of one context at 1; 0 is no identity.
func newIDSequence(mark *idMark) *idSequence {
	return &idSequence{next: 1, marks: []*idMark{mark}}
}

// share hands the sequence to ctx as well, raised past what ctx handed out so far.
func (s *idSequence) share(ctx *Context) {
	s.atLeast(ctx.ids.next)
	ctx.ids.marks = slices.DeleteFunc(ctx.ids.marks, func(m *idMark) bool { return m == ctx.took })
	s.marks = append(s.marks, ctx.took)
	ctx.ids = s
}

// take hands out the next identity to the context marked by mark.
func (s *idSequence) take(mark *idMark) int64 {
	id := s.next
	s.next++
	mark.high = s.next
	return id
}

// atLeast raises the sequence to hand out id next, never lowering it.
func (s *idSequence) atLeast(id int64) {
	if id > s.next {
		s.next = id
	}
}

// release hands out id next again once ctx, rolling back, holds no identity from id
// on — but never one another context sharing the sequence took.
func (s *idSequence) release(ctx *Context, id int64) {
	if ctx.holdsIdentityFrom(id) {
		return
	}
	ctx.took.high = min(ctx.took.high, id)
	next := id
	for _, mark := range s.marks {
		next = max(next, mark.high)
	}
	if next < s.next {
		s.next = next
	}
}

// holdsIdentityFrom reports whether an object, or a connector one set aside,
// holds an identity at or past id.
func (ctx *Context) holdsIdentityFrom(id int64) bool {
	for held, inst := range ctx.instances {
		if held >= id {
			return true
		}
		for _, kept := range inst.keptConnectors {
			if kept >= id {
				return true
			}
		}
		for _, kept := range inst.keptAnonymous {
			if kept.id >= id {
				return true
			}
		}
	}
	return false
}

// heldIdentities are the identities ctx holds: its objects' and those set aside
// for connectors not materialized again.
func (ctx *Context) heldIdentities() map[int64]bool {
	held := make(map[int64]bool, len(ctx.instances))
	for id, inst := range ctx.instances {
		held[id] = true
		for _, kept := range inst.KeptConnectorIDs() {
			held[kept] = true
		}
	}
	return held
}

// allocateID returns the next instance ID and increments the counter.
func (ctx *Context) allocateID() int64 {
	return ctx.ids.take(ctx.took)
}

// claimID counts id as taken by this context: an identity an object was made or
// adopted under rather than handed out here.
func (ctx *Context) claimID(id int64) {
	ctx.ids.atLeast(id + 1)
	ctx.took.high = max(ctx.took.high, id+1)
}

// runState is what one run keeps of itself: the budget it spent, what it noted
// (see note), its scheduler, and the calc usage evaluations of its open activations.
type runState struct {
	steps    int64
	elements int64
	notes    []RunNote
	// scheduler is the resolutions the run's choices draw from (scheduler.go).
	scheduler *scheduler
	// calcUsageRuns holds, per activation under way, the evaluation of each calc
	// usage read in it, so its outputs answer from one run of the body (calc_usage.go).
	calcUsageRuns map[int64]map[calcUsageKey]*calcRun
}

// newRunState is the state a run starts with, under the schedule policy set now.
func (ctx *Context) newRunState() *runState {
	return &runState{
		scheduler:     ctx.newScheduler(),
		calcUsageRuns: make(map[int64]map[calcUsageKey]*calcRun),
	}
}

// enterRun brackets one call of the run with this state: a top-level call installs
// it, and it stays installed after so callers read that run; a nested one shares the outer's.
func (ctx *Context) enterRun(state *runState) func() {
	if ctx.runDepth == 0 {
		ctx.run = state
	}
	ctx.runDepth++
	return ctx.leaveRun
}

// leaveRun ends one call of the run under way; the outermost leaving ends the body
// coroutine kept idle for its steps.
func (ctx *Context) leaveRun() {
	ctx.runDepth--
	if ctx.runDepth == 0 {
		ctx.endIdleBodyCoroutine()
	}
}

// beginRun starts a run and returns the function that ends it: a top-level run
// starts on a fresh state, so the budget bounds one run, not a whole session.
func (ctx *Context) beginRun() func() {
	if ctx.runDepth > 0 {
		return ctx.enterRun(ctx.run)
	}
	return ctx.enterRun(ctx.newRunState())
}

// executorRun is a run driven call by call: its state, nil until its first call
// begins it, which every later call resumes.
type executorRun struct {
	state *runState
}

// beginExecutorRun brackets one call into a call-by-call driven executor: the run's
// own state, fresh at its first call, is installed for each, whatever ran in between.
func (ctx *Context) beginExecutorRun(run *executorRun) func() {
	if run.state == nil {
		if ctx.runDepth > 0 {
			run.state = ctx.run
		} else {
			run.state = ctx.newRunState()
		}
	}
	return ctx.enterRun(run.state)
}

// previewExecutorRun installs, for a preview of a call into a call-by-call driven
// executor, the state that call would run on, restored after; nothing is begun.
func (ctx *Context) previewExecutorRun(run *executorRun) func() {
	if ctx.runDepth > 0 {
		return func() { /* nested: the outer run's state */ }
	}
	saved := ctx.run
	if run.state != nil {
		ctx.run = run.state
	} else {
		ctx.run = ctx.newRunState()
	}
	return func() { ctx.run = saved }
}

// endExecutorRun brackets the release of a call-by-call driven run: its leftovers
// are ended on its own state, nested or not, and the state installed before is restored.
func (ctx *Context) endExecutorRun(run *executorRun) func() {
	if run.state == nil {
		return func() { /* never begun: nothing of its own to end */ }
	}
	saved := ctx.run
	ctx.run = run.state
	return func() { ctx.run = saved }
}

// beginProbe brackets an evaluation previewing what a run would do, restoring the
// budget, trace, bus, variant selections, objects made (identities included),
// behaviors attached, every feature value written (see noteProbeWrite) and every
// other change noted (see noteProbeUndo) after. The writes it makes are not the
// step's (see noteWrite); behaviors it starts are the only ones it runs (see nextRunnableBehavior).
func (ctx *Context) beginProbe() func() {
	run := ctx.run
	steps, elements, trace, writes := run.steps, run.elements, ctx.trace, ctx.stepWrites
	ids, nextID := ctx.ids, ctx.ids.next
	endBoundary := func() { /* no boundary to close */ }
	if ctx.probes == 0 {
		endBoundary = ctx.beginRunBoundary()
	}
	_, rollback := ctx.beginJournal()
	restoreSchedule := run.scheduler.mark()
	ctx.trace, ctx.stepWrites = nil, nil
	ctx.runDepth++
	ctx.probes++
	return func() {
		rollback()
		endBoundary()
		restoreSchedule()
		if ctx.ids == ids {
			ids.release(ctx, nextID)
		}
		ctx.probes--
		ctx.leaveRun()
		run.steps, run.elements, ctx.trace, ctx.stepWrites = steps, elements, trace, writes
	}
}

// runBoundary is where in objectBehaviors and pendingBehaviors the behaviors
// attached since a change began start.
type runBoundary struct {
	behaviors, pending int
}

// beginRunBoundary confines drains to the behaviors attached from now until the
// returned function is called: what an older behavior does cannot be undone.
func (ctx *Context) beginRunBoundary() func() {
	ctx.runBoundaries = append(ctx.runBoundaries, runBoundary{
		behaviors: len(ctx.objectBehaviors),
		pending:   len(ctx.pendingBehaviors),
	})
	return func() { ctx.runBoundaries = ctx.runBoundaries[:len(ctx.runBoundaries)-1] }
}

// beginJournal brackets a change to be kept whole or not at all: the feature
// values written (see noteProbeWrite), the other changes noted (see
// noteProbeUndo — variant selections among them), the bus, and the objects made
// and behaviors attached are journaled until commit keeps them or rollback
// restores them. A commit inside an enclosing journal leaves the entries to it.
func (ctx *Context) beginJournal() (commit, rollback func()) {
	mark := ctx.markJournal()
	ctx.journals++
	commit = func() {
		ctx.journals--
		if ctx.journals == 0 {
			ctx.journalWrites, ctx.journalUndos = ctx.journalWrites[:mark.writes], ctx.journalUndos[:mark.undos]
		}
	}
	rollback = func() {
		ctx.journals--
		ctx.rollbackJournal(mark)
	}
	return commit, rollback
}

// journalWrite is a feature value a journal changed and what it held before.
type journalWrite struct {
	fv    *FeatureValue
	prior FeatureValue
}

// noteProbeWrite records a feature value about to change, for the probe or
// transaction under way to restore; outside one it records nothing.
func (ctx *Context) noteProbeWrite(fv *FeatureValue) {
	if ctx.journals == 0 {
		return
	}
	ctx.journalWrites = append(ctx.journalWrites, journalWrite{fv: fv, prior: *fv})
}

// noteProbeUndo records how to restore state no feature value holds that is about
// to change, for the probe or transaction under way to run; outside one it
// records nothing.
func (ctx *Context) noteProbeUndo(undo func()) {
	if ctx.journals == 0 {
		return
	}
	ctx.journalUndos = append(ctx.journalUndos, undo)
}

// newActivation begins one activation: the identity of a single execution of a
// body, which the values a calc usage answers within it belong to.
func (ctx *Context) newActivation() int64 {
	ctx.activations++
	return ctx.activations
}

// newRun begins one behavior run: the identity a function closing over it carries,
// which no other run of the same behavior shares.
func (ctx *Context) newRun() int64 {
	ctx.runs++
	return ctx.runs
}

// endActivation forgets what an activation computed, once it has ended, and the
// activations of the calc usage evaluations it held.
func (ctx *Context) endActivation(activation int64) {
	runs, ok := ctx.run.calcUsageRuns[activation]
	if !ok {
		return
	}
	delete(ctx.run.calcUsageRuns, activation)
	for _, run := range runs {
		ctx.endActivation(run.activation)
	}
}

// incrementStep increments the step counter and returns ErrStepLimitExceeded if limit reached.
// The error names the effective budget and the variable that raises it.
func (ctx *Context) incrementStep() error {
	ctx.run.steps++
	if ctx.run.steps > ctx.maxSteps {
		return ctx.stepLimitExceeded()
	}
	return nil
}

// stepLimitExceeded reports the step budget spent, naming the variable that raises
// it; kept out of line so the step charge on every evaluation inlines.
//
//go:noinline
func (ctx *Context) stepLimitExceeded() error {
	return fmt.Errorf("%w (%d steps; raise %s to allow more)", ErrStepLimitExceeded, ctx.maxSteps, MaxStepsEnvVar)
}

// elementScope brackets one evaluation and returns the function releasing what
// it materialized, so the bound counts elements held at once, not in total.
func (ctx *Context) elementScope() func() {
	run, held := ctx.run, ctx.run.elements
	return func() { run.elements = held }
}

// beginStep brackets one evaluation outside a body: it answers the activation the
// evaluation runs in and the function ending it, releasing what it materialized.
func (ctx *Context) beginStep() (int64, func()) {
	activation := ctx.newActivation()
	release := ctx.elementScope()
	return activation, func() {
		ctx.endActivation(activation)
		release()
	}
}

// chargeElements counts elements an evaluation materializes, which unlike a step
// is memory the collection holding it keeps, against the element budget.
func (ctx *Context) chargeElements(n int64) error {
	ctx.run.elements += n
	// A count that overflowed is past any budget, so it reads as one.
	if ctx.run.elements > ctx.maxElements || ctx.run.elements < 0 {
		return fmt.Errorf("%w (%d elements; raise %s to allow more)", ErrElementLimitExceeded, ctx.maxElements, MaxElementsEnvVar)
	}
	return nil
}

// Instance retrieves an instance by ID, so a caller holding a ValInstance can
// reach the object it names.
func (ctx *Context) Instance(id int64) (*Instance, bool) {
	return ctx.getInstance(id)
}

// InstanceIDs lists the identities of every object this context holds, in
// ascending order, so a caller can say which ids an unknown one is not among.
func (ctx *Context) InstanceIDs() []int64 {
	ids := make([]int64, 0, len(ctx.instances))
	for id := range ctx.instances {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// getInstance retrieves an instance by ID.
func (ctx *Context) getInstance(id int64) (*Instance, bool) {
	inst, ok := ctx.instances[id]
	return inst, ok
}

// registerInstance stores an instance in the registry.
func (ctx *Context) registerInstance(inst *Instance) {
	if inst.ID <= 0 {
		panic(fmt.Sprintf("runtime: invalid instance ID %d (must be > 0)", inst.ID))
	}
	if _, exists := ctx.instances[inst.ID]; exists {
		panic(fmt.Sprintf("runtime: duplicate instance ID %d", inst.ID))
	}
	ctx.instances[inst.ID] = inst
	ctx.created = append(ctx.created, inst.ID)
}

// EvaluateConstraint evaluates a constraint definition/usage naming no object:
// against the single object of this runtime carrying it, the declared defaults
// when there is none, ErrAmbiguousSubject when there are several.
// Returns (satisfied, error). If IsAssert=true, violation is an error.
// If IsAssert=false (assume), always returns (true, nil) but logs assumptions.
func (ctx *Context) EvaluateConstraint(sym *symbols.Symbol, scope *symbols.Scope) (bool, error) {
	return ctx.EvaluateConstraintOn(sym, scope, nil)
}

// RequireConstraint returns an ErrNotAConstraint usage error unless sym
// declares a constraint, so a caller can settle the kind before evaluating.
func RequireConstraint(sym *symbols.Symbol) error {
	if _, ok := ast.OwnedConstraintOf(sym.Decl); ok {
		return nil
	}
	if ast.ConstraintReferenceOf(sym.Decl) != nil {
		return nil
	}
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		if decl.Kind == ast.DefConstraint {
			return nil
		}
	case *ast.Usage:
		if decl.Kind == ast.UsageConstraint {
			return nil
		}
	}
	return notOfKind(ErrNotAConstraint, sym, "constraint")
}

// RequireRequirement returns an ErrNotARequirement usage error unless sym
// declares a requirement.
func RequireRequirement(sym *symbols.Symbol) error {
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		if decl.Kind == ast.DefRequirement {
			return nil
		}
	case *ast.Usage:
		if decl.Kind == ast.UsageRequirement {
			return nil
		}
	}
	return notOfKind(ErrNotARequirement, sym, "requirement")
}

// EvaluateConstraintOn evaluates a constraint against a concrete instance: a
// feature the constraint names resolves to that instance's feature value, so the same
// constraint can pass for one instance and fail for another. An instance that
// does not carry the constraint itself is searched for the nested object that
// does; a nil instance leaves the subject to EvaluateConstraint's rule.
func (ctx *Context) EvaluateConstraintOn(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) (bool, error) {
	result, err := ctx.CheckConstraintOn(sym, scope, self)
	return result.Holds, err
}

// CheckConstraintOn evaluates a constraint as EvaluateConstraintOn does and also
// reports the object it turned out to be about, which a caller labelling the
// verdict needs: it is not always the instance supplied.
func (ctx *Context) CheckConstraintOn(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) (CheckResult, error) {
	defer ctx.beginRun()()

	if err := RequireConstraint(sym); err != nil {
		return CheckResult{Subject: self}, err
	}
	subject, err := ctx.checkSubject("constraint", sym.Name, sym, self)
	if err != nil {
		return CheckResult{}, err
	}

	// Evaluate every condition the constraint states, inherited ones included.
	conds := ctx.conditionsOf(sym, ctx.chainMembers(sym, scope))
	holds, err := ctx.evaluateConditions(conditionCheck{
		sym:     sym,
		kind:    "constraint",
		what:    "assertion",
		self:    subject.instance,
		negated: NegatedDecl(sym),
	}, conds)
	return ctx.checkResultOf(holds, subject), err
}

// CheckResult is the outcome of one check: whether it holds, the object its
// conditions were evaluated against — nil when they were evaluated against the
// declaration because no object carries the checked element — and, for a nested
// subject, the object the search started from plus the features walked from it,
// one name a segment — ending in the declaration the object materializes, as an
// ambiguity names it — which are how a caller names an object holding no name of
// its own.
type CheckResult struct {
	Holds       bool
	Subject     *Instance
	SubjectRoot *Instance
	SubjectPath []string
}

// checkResultOf reports a verdict about the object a check resolved to.
func (ctx *Context) checkResultOf(holds bool, subject carrier) CheckResult {
	return CheckResult{
		Holds:       holds,
		Subject:     subject.instance,
		SubjectRoot: subject.root,
		SubjectPath: ctx.carrierFeatures(subject),
	}
}

// memberBindings evaluates the values members bind by name — a subject or actor
// supplied by an expression (`actor operator = limit;`) — so a condition naming
// one reads it. kind and element name the checked element in messages. A non-nil
// subject is the object supplied from outside (the `by` of a satisfaction
// assertion): it binds every subject the members declare, whose own binding is
// then neither evaluated nor used. Values are held to their member's effective
// declaration (holdBound) in one transaction, so a refused binding leaves nothing
// behind. enclosing are the values bound around the element (a case run's, for its
// objective), which the binding expressions read.
func (ctx *Context) memberBindings(sym *symbols.Symbol, kind, element string, members []scopedMember, self *Instance, subject *Instance, enclosing frame) (map[string]Value, error) {
	bindings := make(map[string]Value)
	features := ctx.conditionFeatures(sym)
	// The bindings are evaluated as one, so a calc usage two of them read answers
	// from one evaluation, and the next check reads it again.
	activation, endStep := ctx.beginStep()
	defer endStep()
	evalIn := func(memberScope *symbols.Scope) *EvalContext {
		ec := NewEvalContextIn(ctx, memberScope, self)
		ec.activation = activation
		ec.features = features
		if enclosing.vars != nil {
			ec.pushFrame(enclosing)
		}
		ec.Push(bindings)
		return ec
	}
	superseded := ctx.redefinedAmong(sym, members)
	hold := func(member scopedMember, what string, value Value) error {
		if memberSym := memberSymbol(member.scope, member.node); memberSym == nil || superseded[memberSym] {
			return nil
		}
		return ctx.holdBound(sym, member, fmt.Sprintf("%s %s: %s", kind, element, what), value)
	}

	commit, rollback := ctx.beginJournal()
	for _, member := range members {
		var what string
		var names []string
		var expr ast.Node
		isSubject := false
		switch rm := member.node.(type) {
		case *ast.SubjectMember:
			what, names, expr, isSubject = "subject", ctx.memberNames(sym, member, rm.Ident.Name, rm.Ident.ShortName), rm.BindingExpr, true
		case *ast.Usage:
			switch rm.Kind {
			case ast.UsageSubject:
				what, names, expr, isSubject = "subject", ctx.memberNames(sym, member, effectiveName(rm), rm.Ident.ShortName), rm.Value, true
			case ast.UsageActor:
				what, names, expr = "actor", ctx.memberNames(sym, member, effectiveName(rm), rm.Ident.ShortName), rm.Value
			}
		default:
			continue
		}
		if isSubject && subject != nil {
			value := Value{Kind: ValInstance, Instance: subject.ID}
			if err := hold(member, what, value); err != nil {
				rollback()
				return nil, err
			}
			for _, name := range names {
				bindings[name] = value
			}
			continue
		}
		if expr == nil {
			// A redeclaration valuing nothing reads the value the feature it
			// redefines binds, under its own names too.
			if value, ok := boundUnder(bindings, names); ok {
				if err := hold(member, what+" binding", value); err != nil {
					rollback()
					return nil, err
				}
				for _, name := range names {
					bindings[name] = value
				}
			}
			continue
		}
		value, err := evalIn(member.scope).Eval(expr)
		if err != nil {
			rollback()
			return nil, fmt.Errorf("%s %s: %s binding evaluation failed: %w", kind, element, what, err)
		}
		if err := hold(member, what+" binding", value); err != nil {
			rollback()
			return nil, err
		}
		for _, name := range names {
			bindings[name] = value
		}
	}
	commit()
	return bindings, nil
}

// redefinedAmong is the set of members of owner another of members redefines: their
// declarations are superseded by the redefining member's, which holds the value.
func (ctx *Context) redefinedAmong(owner *symbols.Symbol, members []scopedMember) map[*symbols.Symbol]bool {
	superseded := make(map[*symbols.Symbol]bool)
	for _, member := range members {
		memberSym := memberSymbol(member.scope, member.node)
		if memberSym == nil {
			continue
		}
		for _, redefined := range ctx.redefinedFeatures(memberSym, owner) {
			superseded[redefined] = true
		}
	}
	return superseded
}

// holdBound holds val as the value of a bound member of owner: itself and the features it
// redefines, checked against their declaration folded together (see holdAs).
func (ctx *Context) holdBound(owner *symbols.Symbol, member scopedMember, what string, val Value) error {
	memberSym := memberSymbol(member.scope, member.node)
	if memberSym == nil {
		return nil
	}
	features := append([]*symbols.Symbol{memberSym}, ctx.redefinedFeatures(memberSym, owner)...)
	return ctx.holdAs(member.scope, what, ctx.boundMemberDecl(owner, features), val, features...)
}

// holdAs checks val against decl's multiplicity and type, then classifies its objects by each of
// features as one transaction, as a declared feature value is held (KerML §7.3.4.1); what names the binding.
func (ctx *Context) holdAs(scope *symbols.Scope, what string, decl calcMemberDecl, val Value, features ...*symbols.Symbol) error {
	if err := decl.admits(ctx, scope, what, val); err != nil {
		return err
	}
	commit, rollback := ctx.beginJournal()
	for _, feature := range features {
		if err := ctx.classifyHeld(feature, val); err != nil {
			rollback()
			return fmt.Errorf("%s: %w", what, err)
		}
	}
	commit()
	return nil
}

// memberNames are the names a condition may read a bound member of owner by: its
// own and those of every feature it redefines, one feature with it (KerML §7.3.4.5).
func (ctx *Context) memberNames(owner *symbols.Symbol, member scopedMember, name, shortName string) []string {
	names := bindingNames(name, shortName)
	memberSym := memberSymbol(member.scope, member.node)
	if memberSym == nil {
		return names
	}
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		seen[n] = true
	}
	for _, redefined := range ctx.redefinedFeatures(memberSym, owner) {
		for _, n := range bindingNames(redefined.Name, redefined.ShortName) {
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	return names
}

// boundUnder returns the value bindings hold under any of names.
func boundUnder(bindings map[string]Value, names []string) (Value, bool) {
	for _, name := range names {
		if value, ok := bindings[name]; ok {
			return value, true
		}
	}
	return Value{}, false
}

// bindingNames are the names a condition may read a bound member by: its name
// and its short name, whichever it declares.
func bindingNames(name, shortName string) []string {
	var names []string
	if name != "" {
		names = append(names, name)
	}
	if shortName != "" && shortName != name {
		names = append(names, shortName)
	}
	return names
}

// effectiveName is the name a usage answers to, which for a member written as a
// reference is its reference's rather than a declared one (ast.EffectiveName).
func effectiveName(u *ast.Usage) string {
	name, _ := ast.EffectiveName(u)
	return name
}

// NegatedDecl reports whether sym's declaration asserts that its conditions do
// not hold (`assert not constraint { … }`, `assert not satisfy … by …`).
func NegatedDecl(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.IsNegated
}

// scopedMember is a declaration member with the scope it was written in, since
// an inherited member's names resolve where its supertype was declared.
type scopedMember struct {
	node  ast.Node
	scope *symbols.Scope
}

// chainMembers returns the members of the types sym takes members from (its
// supertypes and the feature it references), most general first, then sym's own.
// The library's frame states no model conditions and contributes none; a domain
// library's supertype contributes as a model's does.
func (ctx *Context) chainMembers(sym *symbols.Symbol, scope *symbols.Scope) []scopedMember {
	var out []scopedMember
	supers := ctx.model.semantics.MemberSources(sym)
	for i := len(supers) - 1; i >= 0; i-- {
		link := supers[i]
		if link == nil || ctx.frameDeclared(link) {
			continue
		}
		for _, node := range declMembers(link.Decl) {
			out = append(out, scopedMember{node: node, scope: bodyScope(link, link.OwnerScope)})
		}
	}
	for _, node := range declMembers(sym.Decl) {
		out = append(out, scopedMember{node: node, scope: bodyScope(sym, scope)})
	}
	return out
}

// bodyScope is the scope a member of sym's body was written in: sym's own body,
// where its sibling declarations answer a name before the enclosing namespace
// does (KerML 8.2.3.5.4). fallback covers a declaration that owns no scope.
func bodyScope(sym *symbols.Symbol, fallback *symbols.Scope) *symbols.Scope {
	if sym != nil && sym.Scope != nil {
		return sym.Scope
	}
	return fallback
}

// EvaluateRequirement evaluates a requirement definition/usage naming no object,
// choosing its subject as EvaluateConstraint does.
// Returns (satisfied, error). Validates subject/actor types and evaluates assume/require expressions.
// Assume members always pass (trusted), require members must evaluate to true.
func (ctx *Context) EvaluateRequirement(sym *symbols.Symbol, scope *symbols.Scope) (bool, error) {
	return ctx.EvaluateRequirementOn(sym, scope, nil)
}

// EvaluateRequirementOn evaluates a requirement against a concrete instance,
// binding the features it names to that instance's feature values. The subject is chosen
// as EvaluateConstraintOn chooses it, and the subject/actor bindings are
// evaluated against that same object.
func (ctx *Context) EvaluateRequirementOn(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) (bool, error) {
	result, err := ctx.CheckRequirementOn(sym, scope, self)
	return result.Holds, err
}

// CheckRequirementOn evaluates a requirement as EvaluateRequirementOn does and
// also reports the object it turned out to be about.
func (ctx *Context) CheckRequirementOn(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) (CheckResult, error) {
	defer ctx.beginRun()()

	if err := RequireRequirement(sym); err != nil {
		return CheckResult{Subject: self}, err
	}
	subject, err := ctx.checkSubject("requirement", sym.Name, sym, self)
	if err != nil {
		return CheckResult{}, err
	}

	// Requirement-local bindings are shared by every member, whichever scope it
	// was declared in.
	members := ctx.chainMembers(sym, scope)

	// First pass: process subject/actor bindings
	reqBindings, err := ctx.memberBindings(sym, "requirement", sym.Name, members, subject.instance, nil, frame{})

	if err != nil {
		return ctx.checkResultOf(false, subject), err
	}

	// Second pass: evaluate the assumed and required conditions.
	conds := ctx.conditionsOf(sym, members)
	holds, err := ctx.evaluateConditions(conditionCheck{
		sym:      sym,
		kind:     "requirement",
		what:     "require condition",
		self:     subject.instance,
		bindings: mapFrame(reqBindings),
		negated:  NegatedDecl(sym),
	}, conds)
	if err != nil {
		err = unboundSubjectError(err, "requirement", sym.Name, ctx.unboundSubjectNames(sym, members, subject.instance))
	}
	return ctx.checkResultOf(holds, subject), err
}

// unboundSubjectNames are the subjects the members declare that nothing supplies
// a value for: no binding expression, no object supplied from outside.
func (ctx *Context) unboundSubjectNames(sym *symbols.Symbol, members []scopedMember, subject *Instance) map[string]bool {
	if subject != nil {
		return nil
	}
	names := make(map[string]bool)
	for _, member := range members {
		switch rm := member.node.(type) {
		case *ast.SubjectMember:
			if rm.BindingExpr == nil {
				for _, name := range ctx.memberNames(sym, member, rm.Ident.Name, rm.Ident.ShortName) {
					names[name] = true
				}
			}
		case *ast.Usage:
			if rm.Kind == ast.UsageSubject {
				for _, name := range ctx.memberNames(sym, member, effectiveName(rm), rm.Ident.ShortName) {
					names[name] = true
				}
			}
		}
	}
	return names
}

// unboundSubjectError reports a condition that read an unbound subject as such,
// rather than as a feature that happens to carry no value.
func unboundSubjectError(err error, kind, element string, unbound map[string]bool) error {
	var noValue *NoValueError
	if !errors.As(err, &noValue) || !unbound[noValue.Feature] {
		return err
	}
	return &UnboundSubjectError{Kind: kind, Element: element, Subject: noValue.Feature}
}

// ExecuteAction executes an action definition/usage to completion.
// Returns the values the action's features hold when it completed.
func (ctx *Context) ExecuteAction(action *symbols.Symbol) (map[string]Value, error) {
	return ctx.ExecuteActionWithInputs(action, nil)
}

// ExecuteActionWithInputs executes an action, seeding its feature space with the
// provided input parameter bindings (keyed by parameter name). Inputs override
// action attribute defaults of the same name. Returns the final feature values.
func (ctx *Context) ExecuteActionWithInputs(action *symbols.Symbol, inputs map[string]Value) (map[string]Value, error) {
	return ctx.ExecuteActionPerformedBy(action, nil, inputs)
}

// ExecuteActionPerformedBy executes an action performed by self, whose
// connections route what the action sends and whose variant selections decide
// which of them are realized. A nil self performs the action outside any object.
func (ctx *Context) ExecuteActionPerformedBy(action *symbols.Symbol, self *Instance, inputs map[string]Value) (map[string]Value, error) {
	exec, err := ctx.performAction(action, self, inputs)
	if err != nil {
		return nil, err
	}
	// Return the values the action's features hold once it completed
	return exec.Results(), nil
}

// performAction runs action to completion, performed by self, and returns the
// executor that ran it, whose root performance holds what it produced.
func (ctx *Context) performAction(action *symbols.Symbol, self *Instance, inputs map[string]Value) (*ActionExecutor, error) {
	return ctx.performActionFrom(action, action, self, inputs, (*ActionExecutor).initialize)
}

// performActionStep runs action as a step of an enclosing behavior, the step as named
// being performed. A step stating no flow performs none: it takes its inputs, binds
// its computed outputs and ends at once, as an object performing such an action does.
func (ctx *Context) performActionStep(performed, action *symbols.Symbol, self *Instance, inputs map[string]Value) (*ActionExecutor, error) {
	return ctx.performActionFrom(performed, action, self, inputs, func(exec *ActionExecutor) error {
		if !exec.hasFlow() {
			return exec.completeWithoutFlow()
		}
		return exec.initialize()
	})
}

// performActionFrom creates the executor for a performance of performed running
// action, seeds its inputs, starts it with start, and runs it to completion; the
// clock drives it no further.
func (ctx *Context) performActionFrom(performed, action *symbols.Symbol, self *Instance, inputs map[string]Value, start func(*ActionExecutor) error) (*ActionExecutor, error) {
	defer ctx.beginRun()()

	exec, err := newActionExecutorOf(ctx, performed, action, self, nil)
	if err != nil {
		return nil, fmt.Errorf("create action executor: %w", err)
	}
	defer ctx.clock.detach(exec)

	// Bind inputs before initialization so they seed the initial token.
	if len(inputs) > 0 {
		exec.SetInputs(inputs)
	}

	if err := ctx.startAction(exec, start); err != nil {
		return nil, err
	}
	if exec.state == StateCompleted {
		return exec, nil
	}

	if err := exec.RunToCompletion(); err != nil {
		return nil, fmt.Errorf("execute action: %w", err)
	}
	return exec, nil
}

// startAction begins an executor however its action is performed: one a ToolExecution
// annotates, on the action as named or a type of it, is performed by its tool, which
// completes it; any other is begun by begin. Every way of starting an action passes through here.
func (ctx *Context) startAction(exec *ActionExecutor, begin func(*ActionExecutor) error) error {
	if exec.tool != nil {
		if err := exec.performByTool(exec.tool); err != nil {
			return fmt.Errorf("perform action by tool: %w", err)
		}
		return nil
	}
	if err := begin(exec); err != nil {
		return fmt.Errorf("initialize action: %w", err)
	}
	return nil
}

// ExecuteState executes a state machine, processing events until completion or suspension.
// Returns final state data from the state machine's execution.
// Execution stops when:
// - A final state is reached (StateCompleted)
// - Event queue is empty (StateSuspended)
// - Max event processing steps exceeded (error)
func (ctx *Context) ExecuteState(stateMachine *symbols.Symbol) (map[string]Value, error) {
	data, _, err := ctx.ExecuteStateWithEvents(stateMachine, nil)
	return data, err
}

// ExecuteStateWithEvents executes a state machine, first injecting the provided
// signal events (by signal-type name) into the event queue, then processing all
// events until completion or suspension. Returns the final state data and the
// ordered list of visited state names.
func (ctx *Context) ExecuteStateWithEvents(stateMachine *symbols.Symbol, events []string) (map[string]Value, []string, error) {
	return ctx.ExecuteStatePerformedBy(stateMachine, nil, events)
}

// ExecuteStatePerformedBy executes a state machine performed by self, whose
// connections route what the machine sends and whose variant selections decide
// which of them are realized. A nil self performs it outside any object.
func (ctx *Context) ExecuteStatePerformedBy(stateMachine *symbols.Symbol, self *Instance, events []string) (map[string]Value, []string, error) {
	exec, err := ctx.performState(stateMachine, self, events)
	if err != nil {
		return nil, nil, err
	}
	// Return state machine data and the real ordered visit trace
	return exec.StateData(), exec.GetStateVisits(), nil
}

// StateOutcomeWithEvents runs a state machine as ExecuteStateWithEvents does and
// reports where it came to as the outcome an exploration compares.
func (ctx *Context) StateOutcomeWithEvents(stateMachine *symbols.Symbol, events []string) (Outcome, error) {
	exec, err := ctx.performState(stateMachine, nil, events)
	if err != nil {
		return Outcome{}, err
	}
	return exec.Outcome(), nil
}

// performState runs a state machine performed by self to completion or
// suspension, the events injected before it runs, and returns its executor.
func (ctx *Context) performState(stateMachine *symbols.Symbol, self *Instance, events []string) (*StateExecutor, error) {
	defer ctx.beginRun()()

	// Create executor
	exec, err := newStateExecutor(ctx, stateMachine, self)
	if err != nil {
		return nil, fmt.Errorf("create state executor: %w", err)
	}
	defer ctx.clock.detach(exec)

	// Initialize execution (enters initial state)
	if err := exec.initialize(); err != nil {
		return nil, fmt.Errorf("initialize state machine: %w", err)
	}

	// Inject external signal events. Each event name is treated as a signal type
	// with no arguments; matching accept-triggers consume it in order.
	for _, event := range events {
		exec.SendSignal(event, nil)
	}

	if err := exec.RunToCompletion(); err != nil {
		return nil, err
	}
	return exec, nil
}

// CreateActionExecutor creates an action executor without starting execution.
// For REPL debugging - allows step-by-step execution control.
func (ctx *Context) CreateActionExecutor(action *symbols.Symbol) (*ActionExecutor, error) {
	return ctx.CreateActionExecutorFor(action, nil)
}

// CreateActionExecutorFor creates an action executor for an action performed by
// self, without starting execution. An action a ToolExecution annotates has no flow to
// step: its tool is invoked once and the executor returned completed with its outputs.
func (ctx *Context) CreateActionExecutorFor(action *symbols.Symbol, self *Instance) (*ActionExecutor, error) {
	exec, err := newActionExecutor(ctx, action, self)
	if err != nil {
		return nil, fmt.Errorf("create action executor: %w", err)
	}

	if err := ctx.startAction(exec, (*ActionExecutor).initialize); err != nil {
		exec.Release()
		return nil, err
	}

	return exec, nil
}

// CreateStateExecutor creates a state executor without starting execution.
// For REPL debugging - allows step-by-step execution control.
func (ctx *Context) CreateStateExecutor(stateMachine *symbols.Symbol) (*StateExecutor, error) {
	return ctx.CreateStateExecutorFor(stateMachine, nil)
}

// CreateStateExecutorFor creates a state executor for a machine performed by
// self, without starting execution.
func (ctx *Context) CreateStateExecutorFor(stateMachine *symbols.Symbol, self *Instance) (*StateExecutor, error) {
	exec, err := newStateExecutor(ctx, stateMachine, self)
	if err != nil {
		return nil, fmt.Errorf("create state executor: %w", err)
	}

	// Initialize (enters initial state, schedules initial events)
	if err := exec.initialize(); err != nil {
		exec.Release()
		return nil, fmt.Errorf("initialize state machine: %w", err)
	}

	return exec, nil
}
