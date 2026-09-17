package runtime

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// stmtEnv is the environment a body's statements execute in: the behavior's own
// data, plus one frame per body-local block entered. A frame is discarded when
// its block exits, so a name it declares never leaks outward.
type stmtEnv struct {
	data frame
	// enclosing are the frames the behavior's data shadows but its statements
	// still reach, outermost first: the performances around an action node.
	enclosing []frame
	// outer are value maps the body reads but does not declare into, innermost
	// last: the attributes the states enclosing the behavior own.
	outer []map[string]Value
	// perf is the performance the body runs as where data is not its frame — a state
	// behavior's — whose nodes a read of `p.v` names. nil where data is the frame.
	perf   *actionFrame
	frames []map[string]Value
	// unvalued names, per frame, the features a frame declares but holds no value
	// for yet (`out v : Integer;`), which an assignment writes into that frame.
	unvalued []map[string]bool
}

// enter pushes a frame for a block about to run and returns it.
func (env *stmtEnv) enter() map[string]Value {
	frame := make(map[string]Value)
	env.frames = append(env.frames, frame)
	env.unvalued = append(env.unvalued, nil)
	return frame
}

// leave discards the frame the innermost entered block declares into.
func (env *stmtEnv) leave() {
	if len(env.frames) > 0 {
		env.frames = env.frames[:len(env.frames)-1]
		env.unvalued = env.unvalued[:len(env.unvalued)-1]
	}
}

// declareUnvalued marks a name the innermost entered block declares without a value.
func (env *stmtEnv) declareUnvalued(name string) {
	depth := len(env.frames)
	if depth == 0 {
		return
	}
	if env.unvalued[depth-1] == nil {
		env.unvalued[depth-1] = make(map[string]bool)
	}
	env.unvalued[depth-1][name] = true
}

// frameDeclares reports whether the i-th entered block declares name, valued or not.
func (env *stmtEnv) frameDeclares(i int, name string) bool {
	if _, ok := env.frames[i][name]; ok {
		return true
	}
	return env.unvalued[i][name]
}

// declare binds a name the innermost entered block declares, or a name of the
// behavior's own data when no block is entered.
func (env *stmtEnv) declare(name string, value Value) {
	if depth := len(env.frames); depth > 0 {
		env.frames[depth-1][name] = value
		return
	}
	env.data.set(name, value)
}

// holdsLocal reports whether an entered block declares name.
func (env *stmtEnv) holdsLocal(name string) bool {
	for i := len(env.frames) - 1; i >= 0; i-- {
		if env.frameDeclares(i, name) {
			return true
		}
	}
	return false
}

// assignLocal writes to the innermost entered block that declares name and
// reports whether one did: a block-local declaration shadows a name of the
// behavior's own, including one of its output features.
func (env *stmtEnv) assignLocal(name string, value Value) bool {
	for i := len(env.frames) - 1; i >= 0; i-- {
		if env.frameDeclares(i, name) {
			env.frames[i][name] = value
			return true
		}
	}
	return false
}

// values is the values a statement reads: the enclosing frames, overridden by the
// behavior's own, overridden by the blocks entered around it, innermost last.
func (env *stmtEnv) values() map[string]Value {
	merged := make(map[string]Value, env.data.width())
	for _, f := range env.enclosing {
		f.each(func(name string, value Value) { merged[name] = value })
	}
	env.data.each(func(name string, value Value) { merged[name] = value })
	for _, frame := range env.frames {
		maps.Copy(merged, frame)
	}
	return merged
}

// assign writes to the innermost entered block that declares name, or to the
// behavior's data when that holds the name, and reports whether it found one.
// A name neither declares is the host's to decide on.
func (env *stmtEnv) assign(name string, value Value) bool {
	if env.assignLocal(name, value) {
		return true
	}
	if env.data.has(name) {
		env.data.set(name, value)
		return true
	}
	return false
}

// stmtFlow is how a statement list ended: at its last statement, or at a
// `return` that unwinds every block entered up to the host.
type stmtFlow int

const (
	flowNext stmtFlow = iota
	flowReturn
)

// stmtHost is the behavior a statement engine runs statements for: it names
// itself in diagnostics and decides the statements only it can state — sends,
// returns, assignments reaching outside the body, effects.
type stmtHost interface {
	// describe names the host in a diagnostic ("action node step", "calc P::F").
	describe() string
	// send states the message a send statement addresses.
	send(ec *EvalContext, s lower.Send) error
	// assignOuter writes a name no entered block and no body member declares, and
	// every name declaredOutput claims.
	assignOuter(env *stmtEnv, name string, value Value, s lower.Assign) error
	// assignData writes a name the behavior's data already holds.
	assignData(env *stmtEnv, name string, value Value, s lower.Assign) error
	// assignChain writes the feature a chained target names, on the object the
	// chain reaches; a host with no world outside its body rejects it.
	assignChain(ec *EvalContext, s lower.Assign, value Value) error
	// declaredOutput reports whether name is an output feature of the host, whose
	// assignment binds that output for this activation rather than writing a value
	// the body merely holds.
	declaredOutput(name string) bool
	// acceptReturn takes the value a `return` yields.
	acceptReturn(value Value, s lower.Return) error
	// effect states an effect on the world outside the body, over engine's values.
	effect(engine *stmtEngine, s lower.Effect) error
	// performNode runs a nested action a block's flow declares, node of graph,
	// as a performance of its own with engine's block-locals in reach.
	performNode(engine *stmtEngine, graph *lower.ActionGraph, node *ast.Usage) (stmtFlow, error)
	// runFlow runs the token flow a body states of its own (lower.Block.Stated):
	// its successions and control nodes, as the host's own performance.
	runFlow(block lower.Block) (stmtFlow, error)
	// performer is the object running the behavior, nil when it runs outside any
	// object: what the body's names read and write through.
	performer() *Instance
}

// stmtEngine runs lowered body statements for a host: declarations,
// assignments, conditionals, loops and returns, spending one step of the
// context's budget per loop iteration.
type stmtEngine struct {
	ctx  *Context
	host stmtHost
	env  *stmtEnv
	// activation is the execution the statements now running belong to: the
	// behavior's own, and a fresh one per block entry and per loop iteration.
	activation int64
	// scratch is the context evalIn answers with: statements run one after another
	// and none keeps it past its own evaluation, so one serves them all.
	scratch EvalContext
	// frameBuf is the frame stack scratch reads, rebuilt by every evalIn.
	frameBuf []frame
}

// newStmtEngine returns an engine running statements against data — the
// behavior's own values, which its statements read and write.
func newStmtEngine(ctx *Context, host stmtHost, data map[string]Value) *stmtEngine {
	return &stmtEngine{ctx: ctx, host: host, env: &stmtEnv{data: mapFrame(data)}, activation: ctx.newActivation()}
}

// newStmtEngineOver returns an engine whose statements also read outer value
// maps — the attributes of the states enclosing the behavior — innermost last.
func newStmtEngineOver(ctx *Context, host stmtHost, data map[string]Value, outer []map[string]Value) *stmtEngine {
	engine := newStmtEngine(ctx, host, data)
	engine.env.outer = outer
	return engine
}

// newStmtEngineIn returns an engine running statements against data, a frame
// that shadows the enclosing frames its statements still read, outermost first.
func newStmtEngineIn(ctx *Context, host stmtHost, data frame, enclosing []frame) *stmtEngine {
	return &stmtEngine{
		ctx:        ctx,
		host:       host,
		env:        &stmtEnv{data: data, enclosing: enclosing},
		activation: ctx.newActivation(),
	}
}

// finish ends the activation the engine's statements ran in, discarding what the
// calc usages read in them computed.
func (e *stmtEngine) finish() {
	e.ctx.endActivation(e.activation)
}

// evalIn returns an evaluation context resolving names in the scope the
// statement was written in, reading the behavior's data and the frames entered,
// innermost last so a block-local name shadows an outer one.
func (e *stmtEngine) evalIn(scope *symbols.Scope) *EvalContext {
	frames := append(e.frameBuf[:0], e.env.enclosing...)
	frames = append(frames, e.env.data)
	for _, outer := range e.env.outer {
		frames = append(frames, mapFrame(outer))
	}
	if e.env.perf != nil {
		frames = append(frames, performanceFrame(e.env.perf))
	}
	for _, local := range e.env.frames {
		frames = append(frames, mapFrame(local))
	}
	e.frameBuf = frames
	ec := &e.scratch
	*ec = EvalContext{
		ctx:            e.ctx,
		scope:          scope,
		self:           e.host.performer(),
		frames:         frames,
		trace:          e.ctx.trace,
		inBehaviorBody: true,
		activation:     e.activation,
	}
	return ec
}

// engineFrame is the engine of a body that paused, kept with the values and
// activation it runs in until the body is resumed and ends.
type engineFrame struct{ engine *stmtEngine }

func (f *engineFrame) abandon(*Context) { f.engine.finish() }

func (f *engineFrame) clone() bodyFrame {
	engine, env := *f.engine, *f.engine.env
	env.frames, env.unvalued = slices.Clone(env.frames), slices.Clone(env.unvalued)
	engine.env, engine.scratch, engine.frameBuf = &env, EvalContext{}, nil
	return &engineFrame{engine: &engine}
}

// runStatements runs stmts on the engine build makes, or on the one a paused run
// of them kept; the engine's activation ends with the run, so a body stepped many
// times does not hold what every run computed.
func (ctx *Context) runStatements(build func() *stmtEngine, stmts []lower.Statement) (stmtFlow, error) {
	f, resumed, err := popFrame[*engineFrame](ctx)
	if err != nil {
		return flowNext, err
	}
	if !resumed {
		f = &engineFrame{engine: build()}
	}
	flow, err := f.engine.run(stmts)
	if paused(err) {
		ctx.pushPaused(f)
		return flow, err
	}
	f.engine.finish()
	return flow, err
}

// stmtListFrame is where a statement list paused: at its i-th statement, which
// holds the trace level it opened and the elements held before it (elementScope).
type stmtListFrame struct {
	i        int
	run      *runState
	elements int64
}

func (f *stmtListFrame) abandon(*Context) { f.run.elements = f.elements }

func (f *stmtListFrame) clone() bodyFrame { c := *f; return &c }

// run executes statements in declaration order, stopping at a `return`; a body
// pausing in one is re-entered at that statement, one yielding between two at
// the next.
func (e *stmtEngine) run(stmts []lower.Statement) (stmtFlow, error) {
	f, resumed, err := popFrame[*stmtListFrame](e.ctx)
	if err != nil {
		return flowNext, err
	}
	if !resumed {
		f = &stmtListFrame{}
	}
	resumed = resumed && !e.ctx.yieldedHere()
	for ; f.i < len(stmts); f.i++ {
		if err := e.ctx.yieldBody(); err != nil {
			return flowNext, e.ctx.pausing(f, err)
		}
		flow, err := e.statement(stmts[f.i], f, resumed)
		resumed = false
		if err != nil || flow == flowReturn {
			return flow, e.ctx.pausing(f, err)
		}
		if !compound(stmts[f.i]) {
			e.ctx.bodyPerformed()
		}
	}
	return flowNext, nil
}

// compound reports a statement whose own statements, iterations or nodes are the
// steps of a body run one at a time, not the statement as a whole.
func compound(stmt lower.Statement) bool {
	switch stmt.(type) {
	case lower.If, lower.Loop, lower.Block:
		return true
	}
	return false
}

// statement executes one lowered statement, recording it in the trace with the
// evaluations and nested statements it produces underneath it; one paused keeps
// its trace level and elements open until it is resumed and ends.
func (e *stmtEngine) statement(stmt lower.Statement, f *stmtListFrame, resumed bool) (stmtFlow, error) {
	if !resumed {
		if tr := e.ctx.trace; tr != nil {
			tr.RecordStatement(stmtLabel(stmt))
		}
		// A statement's collections live no longer than the statement, so the one after
		// it starts from the elements held before it.
		f.run, f.elements = e.ctx.run, e.ctx.run.elements
	}
	flow, err := e.execute(stmt)
	if paused(err) {
		return flow, err
	}
	f.run.elements = f.elements
	if tr := e.ctx.trace; tr != nil {
		tr.EndStatement()
	}
	return flow, err
}

// execute runs one lowered statement.
func (e *stmtEngine) execute(stmt lower.Statement) (stmtFlow, error) {
	switch s := stmt.(type) {
	case lower.Send:
		return flowNext, e.host.send(e.evalIn(s.Scope), s)
	case lower.Assign:
		if s.Target == "" {
			return flowNext, fmt.Errorf("%s: unsupported assignment target", e.host.describe())
		}
		value, err := e.evalIn(s.Scope).Eval(s.Value)
		if err != nil {
			return flowNext, fmt.Errorf("eval assignment RHS: %w", err)
		}
		// A chained target writes the object its chain reaches, whatever the body
		// declares of the name it starts from, so no host binding applies to it.
		if s.Chain != nil {
			return flowNext, e.host.assignChain(e.evalIn(s.Scope), s, value)
		}
		// An output is bound by the host even when the body's data holds it, so a
		// second binding is reported; a block-local of the name shadows it.
		if e.env.holdsLocal(s.Target) {
			if err := e.ctx.checkBodyWrite(e.host, s, &value); err != nil {
				return flowNext, err
			}
			e.env.assignLocal(s.Target, value)
			return flowNext, nil
		}
		if !e.host.declaredOutput(s.Target) {
			if e.env.data.has(s.Target) {
				return flowNext, e.host.assignData(e.env, s.Target, value, s)
			}
		}
		return flowNext, e.host.assignOuter(e.env, s.Target, value, s)
	case lower.Declare:
		value := Value{Kind: ValNull}
		if s.Value != nil {
			evaluated, err := e.evalIn(s.Scope).Eval(s.Value)
			if err != nil {
				return flowNext, fmt.Errorf("eval declaration %s: %w", s.Name, err)
			}
			if err := e.ctx.checkBodyDeclaration(s.Scope, e.host.describe(), s.Name, &evaluated); err != nil {
				return flowNext, err
			}
			value = evaluated
		}
		e.env.declare(s.Name, value)
		return flowNext, nil
	case lower.DeclareUsage:
		return flowNext, e.declareUsage(s)
	case lower.Return:
		value := Value{Kind: ValNull}
		if s.Value != nil {
			evaluated, err := e.evalIn(s.Scope).Eval(s.Value)
			if err != nil {
				return flowNext, fmt.Errorf("evaluating the returned expression: %w", err)
			}
			value = evaluated
		}
		if err := e.host.acceptReturn(value, s); err != nil {
			return flowNext, err
		}
		return flowReturn, nil
	case lower.If:
		return e.ifStatement(s)
	case lower.Loop:
		return e.loop(s)
	case lower.Block:
		if s.Own {
			return e.runBlock(s)
		}
		return e.block(s)
	case lower.Effect:
		return flowNext, e.host.effect(e, s)
	case lower.Unsupported:
		return flowNext, fmt.Errorf("%w: %s: %s in a body is not executable", ErrStatementNotExecutable, e.host.describe(), s.Description)
	default:
		return flowNext, fmt.Errorf("%s: unsupported statement %T", e.host.describe(), stmt)
	}
}

// declareUsage brings a body-local calc usage into force: the evaluation of it
// this execution of the body reads starts here, so an evaluation of the same
// usage from before the declaration was reached is discarded.
func (e *stmtEngine) declareUsage(stmt lower.DeclareUsage) error {
	sym, err := e.ctx.bodyUsageSymbol(stmt)
	if err != nil {
		return fmt.Errorf("%s: %w", e.host.describe(), err)
	}
	e.ctx.forgetCalcUsage(e.activation, sym)
	return nil
}

// branchFrame is the branch of a conditional a body paused in.
type branchFrame struct{ elseBranch bool }

func (*branchFrame) abandon(*Context) {}

func (f *branchFrame) clone() bodyFrame { c := *f; return &c }

// ifStatement runs the branch its condition selects, or nothing when the
// condition is false and the conditional declared no else branch.
func (e *stmtEngine) ifStatement(stmt lower.If) (stmtFlow, error) {
	f, resumed, err := popFrame[*branchFrame](e.ctx)
	if err != nil {
		return flowNext, err
	}
	if !resumed {
		// The condition is evaluated outside both branches, so neither branch's
		// declarations are visible to it.
		holds, err := e.condition(stmt.Condition, stmt.Scope, "condition of 'if'")
		if err != nil {
			return flowNext, err
		}
		if !holds && stmt.Else == nil {
			e.ctx.bodyPerformed()
			return flowNext, nil
		}
		f = &branchFrame{elseBranch: !holds}
	}
	branch := stmt.Then
	if f.elseBranch {
		branch = *stmt.Else
	}
	flow, err := e.block(branch)
	return flow, e.ctx.pausing(f, err)
}

// blockFrame is a block a body paused in: the frame its declarations went into,
// its activation, and the one around it.
type blockFrame struct {
	locals     map[string]Value
	unvalued   map[string]bool
	activation int64
	outer      int64
}

func (f *blockFrame) abandon(ctx *Context) { ctx.endActivation(f.activation) }

func (f *blockFrame) clone() bodyFrame {
	c := *f
	c.locals, c.unvalued = maps.Clone(f.locals), maps.Clone(f.unvalued)
	return &c
}

// enterBlock enters a frame and an activation for a block about to run, or the
// ones a paused block ran in; leave restores what was around them.
func (e *stmtEngine) enterBlock(f *blockFrame) (leave func()) {
	if f.locals == nil {
		f.locals = e.env.enter()
		f.activation, f.outer = e.ctx.newActivation(), e.activation
	} else {
		e.env.frames = append(e.env.frames, f.locals)
		e.env.unvalued = append(e.env.unvalued, f.unvalued)
	}
	e.activation = f.activation
	return func() {
		f.unvalued = e.env.unvalued[len(e.env.unvalued)-1]
		e.env.leave()
		e.activation = f.outer
	}
}

// block runs a body-local block in a frame and an activation of its own, so a
// calc usage declared in it is evaluated once per execution of the block.
func (e *stmtEngine) block(block lower.Block) (stmtFlow, error) {
	f, _, err := popFrame[*blockFrame](e.ctx)
	if err != nil {
		return flowNext, err
	}
	if f == nil {
		f = &blockFrame{}
	}
	leave := e.enterBlock(f)
	flow, err := e.runBlock(block)
	leave()
	if paused(err) {
		e.ctx.pushPaused(f)
		return flow, err
	}
	e.ctx.endActivation(f.activation)
	return flow, err
}

// runBlock runs a block's statements, or the token flow it states where a member
// of it is an action node rather than a statement: the host's where the body
// states its successions, else its nodes one after another.
func (e *stmtEngine) runBlock(block lower.Block) (stmtFlow, error) {
	switch {
	case block.Graph == nil:
		return e.run(block.Statements)
	case block.Stated:
		return e.host.runFlow(block)
	}
	return e.blockFlow(block)
}

// flowNodeFrame is the node of a block's flow a body paused at.
type flowNodeFrame struct{ node ast.Node }

func (*flowNodeFrame) abandon(*Context) {}

func (f *flowNodeFrame) clone() bodyFrame { c := *f; return &c }

// blockFlow runs a block that is a token flow of its own (lower/block_graph.go):
// a token starts at the block's initial node and passes along the successions the
// block states, running each node it reaches until one succeeds to none; a body
// run one statement at a time yields between two nodes.
func (e *stmtEngine) blockFlow(block lower.Block) (stmtFlow, error) {
	graph := block.Graph
	f, resumed, err := popFrame[*flowNodeFrame](e.ctx)
	if err != nil {
		return flowNext, err
	}
	if !resumed {
		f = &flowNodeFrame{node: graph.Initial}
	}
	resumed = resumed && !e.ctx.yieldedHere()
	for f.node != nil {
		if err := e.ctx.yieldBody(); err != nil {
			return flowNext, e.ctx.pausing(f, err)
		}
		// A node reached spends a step, so a flow that does not end fails the run.
		if !resumed {
			if err := e.ctx.incrementStep(); err != nil {
				return flowNext, err
			}
		}
		flow, err := e.blockNode(graph, f.node, resumed)
		resumed = false
		if err != nil || flow == flowReturn {
			return flow, e.ctx.pausing(f, err)
		}
		e.ctx.bodyPerformed()
		successors := graph.Edges[f.node]
		if len(successors) == 0 {
			return flowNext, nil
		}
		f.node = successors[0].Target
	}
	return flowNext, nil
}

// blockNode runs one node of a block's flow: the host performs an action usage in
// a frame of its own; a run of statements runs in the frame the block entered.
// A node resumed keeps the trace level it opened.
func (e *stmtEngine) blockNode(graph *lower.ActionGraph, node ast.Node, resumed bool) (stmtFlow, error) {
	if graph.StatementRuns[node] {
		return e.run(graph.Bodies[node])
	}
	traced := false
	if tr := e.ctx.trace; tr != nil {
		if name := ActionNodeName(node); name != "" {
			if !resumed {
				tr.RecordStatement("node " + name)
			}
			traced = true
		}
	}
	var flow stmtFlow
	var err error
	if usage, ok := node.(*ast.Usage); ok {
		flow, err = e.host.performNode(e, graph, usage)
	} else {
		flow, err = e.run(graph.Bodies[node])
	}
	if traced && !paused(err) {
		e.ctx.trace.EndStatement()
	}
	return flow, err
}

// enterActivation runs what follows in a new activation and returns the
// function restoring the enclosing one; for a run that cannot pause.
func (e *stmtEngine) enterActivation() func() {
	outer, entered := e.activation, e.ctx.newActivation()
	e.activation = entered
	return func() {
		e.ctx.endActivation(entered)
		e.activation = outer
	}
}

// loopFrame is a loop a body paused in: the frame of its body's declarations, the
// iteration under way with its activation, and the activation around the loop.
type loopFrame struct {
	locals     map[string]Value
	unvalued   map[string]bool
	iteration  int
	activation int64
	outer      int64
	// elements are what a `for` loop iterates over, evaluated once as it is entered.
	elements []Value
}

func (f *loopFrame) abandon(ctx *Context) { ctx.endActivation(f.activation) }

func (f *loopFrame) clone() bodyFrame {
	c := *f
	c.locals, c.unvalued = maps.Clone(f.locals), maps.Clone(f.unvalued)
	return &c
}

// enterLoop enters the frame the loop's body declares into, or re-enters the one a
// paused loop ran in; leave restores what was around it.
func (e *stmtEngine) enterLoop(f *loopFrame) (leave func()) {
	if f.locals == nil {
		f.locals = e.env.enter()
		f.outer = e.activation
	} else {
		e.env.frames = append(e.env.frames, f.locals)
		e.env.unvalued = append(e.env.unvalued, f.unvalued)
	}
	return func() {
		f.unvalued = e.env.unvalued[len(e.env.unvalued)-1]
		e.env.leave()
		e.activation = f.outer
	}
}

// beginIteration opens one iteration of a loop, in an activation of its own so a
// usage read in it binds its inputs from this iteration's values; a paused
// iteration resumed keeps the activation and trace level it opened.
func (e *stmtEngine) beginIteration(f *loopFrame, resumed bool) {
	if !resumed {
		f.iteration++
		if tr := e.ctx.trace; tr != nil {
			tr.RecordLoopIteration(f.iteration)
		}
		f.activation = e.ctx.newActivation()
	}
	e.activation = f.activation
}

// endIteration closes the iteration under way, unless it paused.
func (e *stmtEngine) endIteration(f *loopFrame, err error) {
	if paused(err) {
		return
	}
	e.ctx.endActivation(f.activation)
	e.activation = f.outer
	if tr := e.ctx.trace; tr != nil {
		tr.EndStatement()
	}
}

// loop runs a loop to termination or to the `return` its body reaches. Every
// iteration spends one step of the budget, so a non-terminating loop fails with
// ErrStepLimitExceeded instead of hanging its caller. A body run one statement
// at a time yields between two iterations.
func (e *stmtEngine) loop(stmt lower.Loop) (stmtFlow, error) {
	if stmt.Kind == ast.LoopFor {
		return e.forLoop(stmt)
	}
	f, resumed, err := popFrame[*loopFrame](e.ctx)
	if err != nil {
		return flowNext, err
	}
	if !resumed {
		f = &loopFrame{}
	}
	resumed = resumed && !e.ctx.yieldedHere()
	leave := e.enterLoop(f)
	defer leave()

	for {
		if err := e.ctx.yieldBody(); err != nil {
			return flowNext, e.ctx.pausing(f, err)
		}
		if !resumed {
			if err := e.ctx.incrementStep(); err != nil {
				return flowNext, err
			}
		}
		flow, done, err := e.iteration(stmt, f, resumed)
		resumed = false
		// A first iteration ended by the condition is the loop's one step; a later
		// one ended so performed nothing since the yield before it.
		if done && err == nil && f.iteration == 1 {
			e.ctx.bodyPerformed()
		}
		if err != nil || done || flow == flowReturn {
			return flow, e.ctx.pausing(f, err)
		}
		e.ctx.bodyPerformed()
	}
}

// iteration runs one iteration of a conditional loop, reporting whether the
// loop's condition ended it; one resumed goes on with its body.
func (e *stmtEngine) iteration(stmt lower.Loop, f *loopFrame, resumed bool) (flow stmtFlow, done bool, err error) {
	e.beginIteration(f, resumed)
	defer func() { e.endIteration(f, err) }()

	if !resumed {
		if stmt.Kind == ast.LoopWhile {
			holds, err := e.condition(stmt.Condition, stmt.Body.Scope, "condition of 'while'")
			if err != nil {
				return flowNext, true, err
			}
			if !holds {
				return flowNext, true, nil
			}
		}
		clear(f.locals)
	}
	flow, err = e.runBlock(stmt.Body)
	if err != nil || flow == flowReturn {
		return flow, true, err
	}

	// An `until` condition is tested after the iteration: the `loop` form keeps
	// it in Condition, a `while` loop carrying one keeps it in Until.
	until := stmt.Until
	if stmt.Kind == ast.LoopUntil {
		until = stmt.Condition
	}
	if until != nil {
		holds, err := e.condition(until, stmt.Body.Scope, "condition of 'until'")
		if err != nil {
			return flowNext, true, err
		}
		return flowNext, holds, nil
	}
	return flowNext, false, nil
}

// forLoop runs the body once per element of the loop's collection, with the
// element bound to the loop's variable in the body's own frame.
func (e *stmtEngine) forLoop(stmt lower.Loop) (stmtFlow, error) {
	if stmt.Variable == "" {
		return flowNext, fmt.Errorf("%s: 'for' loop declares no iteration variable", e.host.describe())
	}
	f, resumed, err := popFrame[*loopFrame](e.ctx)
	if err != nil {
		return flowNext, err
	}
	if !resumed {
		// The collection is evaluated once, before the loop is entered, so the
		// iteration is over the value the loop started with.
		value, err := e.evalIn(stmt.Scope).Eval(stmt.Collection)
		if err != nil {
			return flowNext, fmt.Errorf("eval 'for' collection: %w", err)
		}
		elements, err := forElements(value)
		if err != nil {
			return flowNext, fmt.Errorf("%s: %w", e.host.describe(), err)
		}
		f = &loopFrame{elements: elements}
		if len(elements) == 0 {
			e.ctx.bodyPerformed()
		}
	}
	resumed = resumed && !e.ctx.yieldedHere()
	leave := e.enterLoop(f)
	defer leave()

	for f.iteration < len(f.elements) || resumed {
		if err := e.ctx.yieldBody(); err != nil {
			return flowNext, e.ctx.pausing(f, err)
		}
		if !resumed {
			if err := e.ctx.incrementStep(); err != nil {
				return flowNext, err
			}
		}
		flow, err := e.forIteration(stmt, f, resumed)
		resumed = false
		if err != nil || flow == flowReturn {
			return flow, e.ctx.pausing(f, err)
		}
		e.ctx.bodyPerformed()
	}
	return flowNext, nil
}

// forIteration runs the body once for the next element of the loop's collection;
// one resumed goes on with its body.
func (e *stmtEngine) forIteration(stmt lower.Loop, f *loopFrame, resumed bool) (flow stmtFlow, err error) {
	e.beginIteration(f, resumed)
	defer func() { e.endIteration(f, err) }()
	if !resumed {
		clear(f.locals)
		f.locals[stmt.Variable] = f.elements[f.iteration-1]
	}
	return e.runBlock(stmt.Body)
}

// stmtLabel names a statement for a trace by what it does and, where it has
// one, the feature or loop variable it acts on.
func stmtLabel(stmt lower.Statement) string {
	switch s := stmt.(type) {
	case lower.Send:
		return "send"
	case lower.Assign:
		if s.Chain != nil {
			return "assign " + s.Chain.Text
		}
		return "assign " + s.Target
	case lower.Declare:
		return "declare " + s.Name
	case lower.DeclareUsage:
		return "declare calc " + s.Name
	case lower.Return:
		return "return"
	case lower.Block:
		return "action body"
	case lower.If:
		return "if"
	case lower.Loop:
		switch s.Kind {
		case ast.LoopFor:
			return "for " + s.Variable
		case ast.LoopUntil:
			return "loop until"
		default:
			return "while"
		}
	case lower.Effect:
		return s.Kind.String()
	case lower.Unsupported:
		return s.Description
	default:
		return fmt.Sprintf("%T", stmt)
	}
}

// forElements returns the elements a `for` loop visits, in visiting order: a
// sequence in the order the expression built it (a range ascending, a filter as
// the collection it filtered), and a set in its canonical order since a set
// carries no order of its own. A `for` input that is not a collection is
// reported rather than read as the one-element collection elementsOf coerces
// it to: iterating a scalar is a modelling error, and a single silent
// iteration hides it.
func forElements(value Value) ([]Value, error) {
	switch value.Kind {
	case ValSequence:
		if value.Sequence() == nil {
			return nil, nil
		}
		return value.Sequence().Elements(), nil
	case ValSet:
		if value.Set() == nil {
			return nil, nil
		}
		return value.Set().Elements(), nil
	case ValNull:
		// An absent value holds no elements, which is the empty collection: zero
		// iterations, not an error.
		return nil, nil
	default:
		return nil, fmt.Errorf("%w: 'for' iterates a collection, and %s is not one",
			ErrTypeMismatch, describeValue(value))
	}
}

// condition evaluates a loop or branch condition. A condition that is not
// Boolean is a type error the typecheck pass reports (passes/typecheck.go
// checkBehaviorMember); an execution that reaches one was never checked, so it
// is reported here rather than coerced.
func (e *stmtEngine) condition(expr ast.Node, scope *symbols.Scope, what string) (bool, error) {
	if expr == nil {
		return false, fmt.Errorf("%s: %s is missing", e.host.describe(), what)
	}
	value, err := e.evalIn(scope).Eval(expr)
	if err != nil {
		return false, fmt.Errorf("eval %s: %w", what, err)
	}
	if value.Kind != ValConst || value.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("%s: %s must evaluate to a Boolean, got %s",
			e.host.describe(), what, value.Kind)
	}
	return value.Const.Bool, nil
}
