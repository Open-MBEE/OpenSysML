package runtime

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A body's work — a token's step of a node with a body, or a state's do behavior —
// pauses where it waits and is resumed later. The paused work is data: the
// frames each level of it was at (bodyRun.cursor), re-entered level by level.

// errPaused unwinds the work of a body to the run driving it, which keeps where it paused.
var errPaused = errors.New("body paused")

// errBodyContinuation is a paused body resumed at a level other than the one it paused at.
var errBodyContinuation = errors.New("body continuation")

// paused reports whether err is the unwinding of a paused body.
func paused(err error) bool { return errors.Is(err, errPaused) }

// bodyWork is what a body run performs; perform is re-entered after each pause,
// clone copies how far it has come, for a snapshot to restore it to, and spell
// writes that into a state's canonical form.
type bodyWork interface {
	perform() error
	clone() bodyWork
	spell(*stateSpeller) string
}

func (ctx *Context) enclosingExecutorStep() int {
	if ctx.body == nil {
		return 0
	}
	switch work := ctx.body.work.(type) {
	case *usageWork:
		return work.exec.stepCount + 1
	case *statementWork:
		return work.exec.stepCount + 1
	case *executionWork:
		return work.exec.stepCount + 1
	default:
		return 0
	}
}

// bodyFrame is where one level of a body's work paused; abandon ends what it
// holds open, clone copies it as it stands, for a snapshot to restore it to, and
// spell writes it into a state's canonical form.
type bodyFrame interface {
	abandon(ctx *Context)
	clone() bodyFrame
	spell(*stateSpeller) string
}

// bodyRun is the work of one body: a breakpoint met inside it, a wait on the
// clock, or a statement boundary where it yields, pauses it there until
// resumed, its frames kept in cursor.
type bodyRun struct {
	work bodyWork
	// err is what ended the work, once ended: its own failure, or its abandonment.
	err   error
	ended bool
	// cursor is the paused work, innermost level first; resuming is the cursor
	// being re-entered, each level popping its frame.
	cursor, resuming []bodyFrame
	// pausedAt orders the paused runs by when they last paused; paused is why.
	pausedAt int64
	paused   bodyPause
	// traceLevels is the trace nesting the paused work holds open, set aside
	// while it is paused so what runs meanwhile records at the outer depth.
	traceLevels int
	// traceBase is the nesting the work resumed at, which its levels count from.
	traceBase int
	// awaitsMessages lets the run pause for a message too, as a do behavior does
	// while its machine goes on; an outer body sets the waits its run may pause for.
	awaitsMessages bool
	// yields has the run pause at the statement boundary after the statement,
	// loop iteration or flow step it performed since resumed, which performed marks.
	yields, performed bool
	// guards has it yield between an `if`'s guard and its branch as well.
	guards bool
	// nodesYield makes a loop or `if` node of the flow the run states yield as a statement of its body does.
	nodesYield bool
	// draws has a seeded run draw whether to yield at each such boundary, by the
	// token the run is for and how many boundaries it drew at before.
	draws    bool
	token    int64
	boundary uint64
	// steps has the run pause after each token move of the flows and actions it
	// drives where a step is one move, its machine going on between the moves;
	// shared only in a flow two of whose moves may touch what another does.
	steps, shared bool
	// holdsClock means a clock wait is the hold's refusal, not a body pause.
	holdsClock bool
	// stepDraws has a seeded run draw whether to pause after a callee's start shot or a
	// move of its flow where two of its moves may touch what another does.
	stepDraws bool
	// lists are the unordered statement lists running, outermost first.
	lists []*listLevel
}

// listLevel is an unordered statement list running in a body: moved is whether its
// statement running made a move since it started or went on.
type listLevel struct {
	frame *stmtListFrame
	order *lower.StatementOrder
	moved bool
}

// enterList notes f running in the body on the stack; nil where none is.
func (ctx *Context) enterList(f *stmtListFrame, order *lower.StatementOrder) *listLevel {
	if ctx.body == nil {
		return nil
	}
	level := &listLevel{frame: f, order: order}
	ctx.body.lists = append(ctx.body.lists, level)
	return level
}

// leaveList notes the innermost list entered, level, done running.
func (ctx *Context) leaveList(level *listLevel) {
	if level == nil {
		return
	}
	lists := ctx.body.lists
	ctx.body.lists = lists[:len(lists)-1]
}

// switchStrand pauses the body on the stack back to the innermost unordered list
// whose statement running has moved and does not commute with one it may set
// aside for; nil, going on, where none is.
func (ctx *Context) switchStrand() error {
	run := ctx.body
	if run == nil {
		return nil
	}
	for k := len(run.lists) - 1; k >= 0; k-- {
		l := run.lists[k]
		f := l.frame
		if f.i < 0 || !l.moved || len(l.order.Rivals(f.i, f.done, f.divided)) == 0 {
			continue
		}
		return ctx.pauseBody(bodyPause{yielded: true, strand: f})
	}
	return nil
}

// bodyPause is why a body run paused: at the breakpoint, on a wait, yielded at a
// statement boundary, or after one token move (tokenStep), to go on when resumed.
type bodyPause struct {
	breakpoint breakpointStop
	onWait     bool
	wait       bodyWait
	yielded    bool
	tokenStep  bool
	// strand is the unordered statement list the pause unwinds to, which sets the
	// statement it unwound from aside rather than pausing the body; nil for none.
	strand *stmtListFrame
}

// bodyWait is the wait a body's run paused on: of the action it performs (held),
// or of perf's flow run by exec; onMessage for a wait for a message, else the clock.
type bodyWait struct {
	held      *ActionExecutor
	exec      *ActionExecutor
	perf      *actionFrame
	onMessage bool
}

// goesOn reports whether the wait goes on, so resuming would only pause again.
func (w bodyWait) goesOn() bool {
	if e := w.held; e != nil {
		if e.state != StateWaiting || e.canProceed(nil) {
			return false
		}
		return w.onMessage && e.waitsForMessage(nil, make(map[waitTarget]bool)) ||
			e.waitsOnClock(nil) || e.watchesChangeIn(nil, make(map[waitTarget]bool))
	}
	e := w.exec
	if e.canProceed(w.perf) {
		return false
	}
	return w.onMessage && e.waitsForMessage(w.perf, make(map[waitTarget]bool)) ||
		e.waitsOnClock(w.perf) || e.watchesChangeIn(w.perf, make(map[waitTarget]bool))
}

// waitsForMessage reports whether the wait ultimately holds a non-clock accept.
func (w bodyWait) waitsForMessage(seen map[waitTarget]bool) bool {
	if !w.onMessage {
		return false
	}
	if w.held != nil {
		return w.held.waitsForMessage(nil, seen)
	}
	if w.exec != nil {
		return w.exec.waitsForMessage(w.perf, seen)
	}
	return false
}

func (w bodyWait) watchesChange(seen map[waitTarget]bool) bool {
	if w.held != nil {
		return w.held.watchesChangeIn(nil, seen)
	}
	if w.exec != nil {
		return w.exec.watchesChangeIn(w.perf, seen)
	}
	return false
}

func (w bodyWait) changeWaitHolds(seen map[waitTarget]bool) bool {
	if w.held != nil {
		return w.held.changeWaitHoldsIn(nil, seen)
	}
	if w.exec != nil {
		return w.exec.changeWaitHoldsIn(w.perf, seen)
	}
	return false
}

// waiter is the executor the paused work waits on, nil for none: the action it
// holds, or the flow of a case it runs, which is not own, the flow of the body's executor.
func (w bodyWait) waiter(own *ActionExecutor) clockWaiter {
	if w.held != nil {
		return w.held
	}
	if w.exec != nil && w.exec != own {
		return w.exec
	}
	return nil
}

// resume lets the work go on to its next pause, reported as true with why, or to
// its end, leaving err set.
func (run *bodyRun) resume(ctx *Context) (bodyPause, bool) {
	outer := ctx.body
	ctx.body = run
	run.resuming, run.cursor = run.cursor, nil
	run.performed = false
	base := ctx.trace.nesting()
	run.traceBase = base
	ctx.trace.setNesting(base + run.traceLevels)
	err := run.work.perform()
	ctx.body = outer
	// Levels a failure left un-entered hold nothing the work still needs.
	for _, f := range run.resuming {
		f.abandon(ctx)
	}
	run.resuming = nil
	if paused(err) {
		run.traceLevels = ctx.trace.nesting() - base
		ctx.trace.setNesting(base)
		return run.paused, true
	}
	run.err, run.ended = err, true
	return bodyPause{}, false
}

// end ends the paused work for good: what its frames hold open is abandoned,
// innermost first, an action it was performing among it; err records the abandonment.
func (run *bodyRun) end(ctx *Context) {
	if run.ended {
		return
	}
	for _, f := range run.cursor {
		f.abandon(ctx)
	}
	run.cursor, run.ended = nil, true
	where := "between statements"
	switch {
	case run.paused.onWait:
		where = "on a wait"
	case run.paused.tokenStep:
		where = "between two moves of its flow"
	case !run.paused.yielded:
		where = fmt.Sprintf("at breakpoint %q", run.paused.breakpoint.name)
	}
	run.err = fmt.Errorf("%w: the run paused %s was abandoned", ErrActionDeadlock, where)
}

// endPerformed ends perf where a body statement of the paused run was performing it,
// abandoning the levels within it: the run resumed goes on past the node as completed.
func (run *bodyRun) endPerformed(ctx *Context, perf *actionFrame) bool {
	rest, pf, inStrand := endPerformedIn(ctx, run.cursor, perf)
	if pf == nil {
		return false
	}
	if !inStrand {
		run.cursor = rest
		run.traceLevels = pf.levels
		run.paused = bodyPause{}
	}
	return true
}

// endPerformedIn finds the frame performing perf in cursor, or in a statement an
// unordered list in it set aside, abandoning the levels within it; rest is what of
// cursor is left, unchanged where the frame was in a statement set aside.
func endPerformedIn(ctx *Context, cursor []bodyFrame, perf *actionFrame) (rest []bodyFrame, ended *performFrame, inStrand bool) {
	for i, f := range cursor {
		switch f := f.(type) {
		case *performFrame:
			if f.perf != perf {
				continue
			}
			for _, inner := range cursor[:i] {
				inner.abandon(ctx)
			}
			f.ended = true
			return cursor[i:], f, false
		case *stmtListFrame:
			for _, s := range f.strands {
				if s == nil {
					continue
				}
				if left, pf, _ := endPerformedIn(ctx, s.cursor, perf); pf != nil {
					s.cursor, s.levels, s.paused = left, pf.levels-f.levels, bodyPause{}
					return cursor, pf, true
				}
			}
		}
	}
	return cursor, nil, false
}

// bodyLevels is the trace nesting the body on the stack holds open at this point
// of its work, over the depth it resumed at; 0 with no body on the stack.
func (ctx *Context) bodyLevels() int {
	if ctx.body == nil {
		return 0
	}
	return ctx.trace.nesting() - ctx.body.traceBase
}

// pushPaused keeps f, where the level of the body's work now unwinding paused.
func (ctx *Context) pushPaused(f bodyFrame) {
	ctx.body.cursor = append(ctx.body.cursor, f)
}

// popFrame takes the frame the level resuming the body's work paused at, false
// where the work is not resuming; a frame of another kind is an error.
func popFrame[T bodyFrame](ctx *Context) (frame T, resumed bool, err error) {
	run := ctx.body
	if run == nil || len(run.resuming) == 0 {
		return frame, false, nil
	}
	top := run.resuming[len(run.resuming)-1]
	frame, ok := top.(T)
	if !ok {
		return frame, false, fmt.Errorf("%w: %T resumed where %T paused", errBodyContinuation, frame, top)
	}
	run.resuming = run.resuming[:len(run.resuming)-1]
	return frame, true, nil
}

// resumingAt reports whether the level re-entering the body's work paused at a
// frame of kind T, which it is about to pop.
func resumingAt[T bodyFrame](ctx *Context) bool {
	run := ctx.body
	if run == nil || len(run.resuming) == 0 {
		return false
	}
	_, ok := run.resuming[len(run.resuming)-1].(T)
	return ok
}

// pausing keeps f for the paused body unwinding through err, which it returns.
func (ctx *Context) pausing(f bodyFrame, err error) error {
	if paused(err) {
		ctx.pushPaused(f)
	}
	return err
}

// syncBoundary runs what follows with no body to pause: a wait under it drives the
// clock itself, or is an error where a behavior holds it. It returns the restorer.
func (ctx *Context) syncBoundary() func() {
	outer := ctx.body
	ctx.body = nil
	return func() { ctx.body = outer }
}

// usageWork is a token's step of a nested action usage: the case it is or the
// action it performs, then its own body, then the succession out of it.
type usageWork struct {
	exec  *ActionExecutor
	token int64
	perf  *actionFrame
	graph *lower.ActionGraph
	usage *ast.Usage
	// inv is the action the usage performs, where performs; isCase marks a case step.
	inv      actionInvocation
	performs bool
	isCase   bool
	phase    usagePhase
}

// usagePhase is how far a usageWork has come.
type usagePhase int

const (
	usagePerforming usagePhase = iota // the case it is or the action it performs
	usageBody                         // the flow it owns or its statements
	usageComplete                     // the succession out of it
)

func (w *usageWork) clone() bodyWork { c := *w; return &c }

func (w *usageWork) perform() error {
	e := w.exec
	var ended *terminated
	if w.phase == usagePerforming {
		switch {
		case w.isCase:
			if err := e.performCase(w.perf); err != nil {
				return err
			}
			if err := e.endPerformance(w.perf); err != nil {
				return err
			}
			w.phase = usageComplete
		case w.performs:
			if err := e.performInvocation(w.perf, w.inv); err != nil {
				return err
			}
			w.phase = usageBody
		default:
			w.phase = usageBody
			if lower.ReadsAtStart(w.graph, w.usage) {
				e.ctx.bodyPerformed()
			}
		}
	}
	if w.phase == usageBody {
		// A node owning a flow performs it as subperformances, completing once they have.
		if w.perf.graph != nil {
			idx, err := e.workToken(w.token)
			if err != nil {
				return err
			}
			return e.enterSubflow(idx, w.perf)
		}
		if err := e.executeBody(w.perf, w.graph, w.usage); err != nil {
			err = e.terminatedUsage(w.perf, w.graph, err)
			if ended = unwound(err); ended == nil {
				return err
			}
			if ended.perf != w.perf {
				// The usage ends a performance around it: its own completes first, pins bound.
				if err := e.endPerformance(w.perf); err != nil {
					return err
				}
				return ended
			}
			// A terminate unwound out of a flow nested in the body: what still runs there is dropped.
			e.dropTokensIn(w.perf, 0)
		}
		if err := e.endPerformance(w.perf); err != nil {
			return err
		}
		w.phase = usageComplete
	}
	idx, err := e.workToken(w.token)
	if err != nil {
		return err
	}
	if err := e.completeNode(idx, w.perf); err != nil {
		return err
	}
	if ended != nil {
		return e.endAlongside(ended)
	}
	return nil
}

// statementWork is a token's step of a node written as a statement: its body, then
// the succession out of it.
type statementWork struct {
	exec  *ActionExecutor
	token int64
	frame *actionFrame
	node  ast.Node
	// step is the node's performance, owning the nodes its body performs.
	step *actionFrame
	done bool
}

func (w *statementWork) clone() bodyWork { c := *w; return &c }

func (w *statementWork) perform() error {
	e := w.exec
	if !w.done {
		if err := e.executeStatementBody(w.step, w.frame.graph); err != nil {
			return err
		}
		w.step.ended = true
		w.done = true
	}
	idx, err := e.workToken(w.token)
	if err != nil {
		return err
	}
	return e.leaveStatementNode(idx, w.frame, w.node)
}

// executionWork is a token's step of an execution node invoking an action: the
// invocation, whose outputs the node's features take, then the succession out of it.
type executionWork struct {
	exec    *ActionExecutor
	token   int64
	frame   *actionFrame
	node    *ast.ActionExecutionNode
	outputs map[string]Value
	invoked bool
}

func (w *executionWork) clone() bodyWork {
	c := *w
	c.outputs = maps.Clone(w.outputs)
	return &c
}

func (w *executionWork) perform() error {
	e := w.exec
	if !w.invoked {
		_, outputs, err := invokeAction(
			e.ctx, w.frame.graph.Scope, actionInvocation{target: w.node.ActionRef}, lexicalValues(w.frame), e.self,
		)
		if err != nil {
			return err
		}
		w.outputs, w.invoked = outputs, true
		if err := e.setFrameFeatures(w.frame, outputs); err != nil {
			return err
		}
	}
	idx, err := e.workToken(w.token)
	if err != nil {
		return err
	}
	return e.leaveExecutionNode(idx, w.frame, w.node)
}

// workToken is the index of the token whose step is under way; one gone is an error.
func (e *ActionExecutor) workToken(id int64) (int, error) {
	idx := e.tokenIndex(id)
	if idx < 0 {
		return -1, fmt.Errorf("%w: token %d left its step", errBodyContinuation, id)
	}
	return idx, nil
}

// bodyDivides reports whether another performance may interleave inside work's
// body with an effect on an outcome, so a run going one move at a time yields in it;
// open where moves outside its flow may interleave too.
func (e *ActionExecutor) bodyDivides(work bodyWork, open bool) bool {
	var graph *lower.ActionGraph
	var node ast.Node
	switch w := work.(type) {
	case *usageWork:
		if w.performs {
			return false
		}
		graph, node = w.graph, w.usage
	case *statementWork:
		graph, node = w.frame.graph, w.node
	default:
		return false
	}
	if graph == nil {
		return false
	}
	key := bodyDivision{node: node, open: open}
	divides, known := e.divides[key]
	if !known {
		divides = lower.BodyDivides(graph, node) || open && lower.BodySharesMoves(graph, node)
		if e.divides == nil {
			e.divides = make(map[bodyDivision]bool)
		}
		e.divides[key] = divides
	}
	return divides
}

// bodyDivision keys the cache of bodyDivides.
type bodyDivision struct {
	node ast.Node
	open bool
}

// runBody starts work for the token at tokenIdx and drives it to its first pause or end.
// A run one move at a time with another move open goes one move at a time inside
// the work too, through the flows and actions it performs (stepsTokens).
func (e *ActionExecutor) runBody(tokenIdx int, work bodyWork) error {
	run := &bodyRun{work: work, awaitsMessages: true}
	if outer := e.ctx.body; outer != nil {
		run.awaitsMessages, run.steps, run.shared = outer.awaitsMessages, outer.steps, outer.shared
		run.stepDraws, run.token = outer.stepDraws, outer.token
	}
	open := run.steps
	scheduling := e.ctx.scheduling()
	if scheduling.oneMove() && len(e.tokens) > 1 && !run.steps {
		run.steps, run.shared = true, true
	}
	if _, draws := scheduling.bodyYields(len(e.tokens) > 1); draws && !run.stepDraws {
		run.stepDraws, run.token = true, e.tokens[tokenIdx].ID
	}
	if yields, draws := scheduling.bodyYields(len(e.tokens) > 1 || open); yields && (open || !e.tokens[tokenIdx].drivenByBody()) && e.bodyDivides(work, open) {
		run.yields, run.draws, run.token, run.guards = yields, draws, e.tokens[tokenIdx].ID, true
	}
	if outer := e.ctx.body; outer != nil && outer.nodesYield && yieldsAsStatement(work) {
		run.yields, run.nodesYield = true, true
		run.guards = run.guards || outer.guards
	}
	e.tokens[tokenIdx].body = run
	return e.resumeBody(tokenIdx)
}

// yieldsAsStatement reports work that is a loop or `if` written as a node of a flow.
func yieldsAsStatement(work bodyWork) bool {
	w, ok := work.(*statementWork)
	if !ok {
		return false
	}
	switch w.node.(type) {
	case *ast.WhileLoopActionNode, *ast.IfActionNode:
		return true
	}
	return false
}

// Release ends the run for good: the work of every token a breakpoint left
// paused is ended, so an executor abandoned mid-run holds no suspended run, the
// clock drives it no further, and a later Step or RunToCompletion returns
// ErrExecutorReleased, completed or not. Safe to call more than once.
func (e *ActionExecutor) Release() {
	defer e.ctx.endExecutorRun(&e.driven)()
	e.released = true
	e.endPausedBodies()
	e.ctx.clock.detach(e)
}

// endPausedBodies ends the work of every token a breakpoint left paused; a run
// that fails ends it too, as no step of it goes on.
func (e *ActionExecutor) endPausedBodies() {
	for i := range e.tokens {
		if run := e.tokens[i].body; run != nil {
			run.end(e.ctx)
			e.tokens[i].body = nil
		}
	}
}

// pausedTokens returns the IDs of the tokens Step resumes, whose work paused,
// the longest paused first; a body's flow resumes the tokens it drives.
func (e *ActionExecutor) pausedTokens() []int64 {
	var paused []Token
	for _, token := range e.tokens {
		if token.body != nil && !token.drivenByBody() {
			paused = append(paused, token)
		}
	}
	slices.SortFunc(paused, func(a, b Token) int {
		return cmp.Compare(a.body.pausedAt, b.body.pausedAt)
	})
	ids := make([]int64, len(paused))
	for i, token := range paused {
		ids[i] = token.ID
	}
	return ids
}

// tokenIndex returns the index of the token with the given ID, -1 for none.
func (e *ActionExecutor) tokenIndex(id int64) int {
	for i, token := range e.tokens {
		if token.ID == id {
			return i
		}
	}
	return -1
}

// resumeBody lets the paused work of the token at tokenIdx go on to its next pause
// (a breakpoint suspends the executor, the clock does not) or to its end.
func (e *ActionExecutor) resumeBody(tokenIdx int) error {
	run, id := e.tokens[tokenIdx].body, e.tokens[tokenIdx].ID
	if pause, paused := run.resume(e.ctx); paused {
		e.pauses++
		run.pausedAt = e.pauses
		switch {
		case pause.yielded || pause.tokenStep:
			if i := e.tokenIndex(id); i >= 0 {
				e.tokens[i].moved = e.sweep
			}
		case !pause.onWait:
			e.pausedAt = pause.breakpoint
			e.state = StateSuspended
		}
		return nil
	}
	if i := e.tokenIndex(id); i >= 0 {
		e.tokens[i].body = nil
	}
	return run.err
}

// pauseAt pauses the run before a node a breakpoint is set on performs, as the run
// pauses before a token steps such a node; a run not made pausable goes on.
func (e *ActionExecutor) pauseAt(within []ast.Node, node ast.Node) error {
	if stop, set := e.stopAt(within, node); set {
		return e.ctx.pauseBody(bodyPause{breakpoint: stop})
	}
	return nil
}

// pauseBody pauses the body on the stack, unwinding its work to the run driving
// it; nil, going on, where no body is on the stack.
func (ctx *Context) pauseBody(pause bodyPause) error {
	run := ctx.body
	if run == nil {
		return nil
	}
	run.paused = pause
	return errPaused
}

// yieldBody pauses the body on the stack before its next statement where its run
// goes one at a time and has performed one since resumed; nil, going on, else.
func (ctx *Context) yieldBody() error {
	if err := ctx.switchStrand(); err != nil {
		return err
	}
	if ctx.body == nil || !ctx.body.yields || !ctx.body.performed {
		return nil
	}
	if ctx.body.draws {
		ctx.body.boundary++
		if !ctx.scheduling().drawYield(ctx.body.token, ctx.body.boundary) {
			return nil
		}
	}
	return ctx.pauseBody(bodyPause{yielded: true})
}

// bodyPerformed notes a statement, loop iteration or flow step of the body on the
// stack done, after which a run going one at a time yields.
func (ctx *Context) bodyPerformed() {
	if ctx.body != nil {
		ctx.body.performed = true
		ctx.body.listsMoved()
	}
}

// listsMoved notes a move made by the statement each unordered list running runs.
func (run *bodyRun) listsMoved() {
	for _, l := range run.lists {
		l.moved = true
	}
}

// stepsTokens reports whether the body on the stack pauses after each token move.
func (ctx *Context) stepsTokens() bool {
	return ctx.body != nil && ctx.body.steps
}

// guardPerformed notes an `if`'s guard read by the body on the stack, after which a
// run yielding between a guard and its branch yields.
func (ctx *Context) guardPerformed() {
	if ctx.body == nil {
		return
	}
	ctx.body.listsMoved()
	if ctx.body.guards {
		ctx.body.performed = true
	}
}

// tokenStepBody pauses the body on the stack after one token move of graph's flow
// where its run goes one move at a time there, or a seeded draw says so; nil, going on, else.
func (ctx *Context) tokenStepBody(graph *lower.ActionGraph) error {
	switch {
	case ctx.stepsTokens():
		if ctx.body.shared && !ctx.flowSharesMoves(graph) {
			return nil
		}
	case ctx.drawsTokenSteps():
		if !ctx.flowSharesMoves(graph) {
			return nil
		}
		ctx.body.boundary++
		if !ctx.scheduling().drawYield(ctx.body.token, ctx.body.boundary) {
			return nil
		}
	default:
		return nil
	}
	return ctx.pauseBody(bodyPause{tokenStep: true})
}

// drawsTokenSteps reports whether the body on the stack is a seeded run drawing
// whether to pause after the moves of the callees it performs.
func (ctx *Context) drawsTokenSteps() bool {
	return ctx.body != nil && ctx.body.stepDraws
}

// flowSharesMoves caches lower.FlowSharesMoves by graph.
func (ctx *Context) flowSharesMoves(graph *lower.ActionGraph) bool {
	if graph == nil {
		return false
	}
	shares, known := ctx.flowShares[graph]
	if !known {
		shares = lower.FlowSharesMoves(graph)
		if ctx.flowShares == nil {
			ctx.flowShares = make(map[*lower.ActionGraph]bool)
		}
		ctx.flowShares[graph] = shares
	}
	return shares
}

// yieldedHere reports the frame just popped as the one the body yielded in: its
// next statement begins afresh there, where a frame paused inside one resumes it.
func (ctx *Context) yieldedHere() bool {
	run := ctx.body
	return run != nil && len(run.resuming) == 0 && run.paused.yielded
}

// pauseForClock pauses the body on the stack while wait, a wait on the clock,
// goes on; nil where none is on the stack.
func (ctx *Context) pauseForClock(wait bodyWait) error {
	if ctx.body != nil && ctx.body.holdsClock {
		return nil
	}
	return ctx.pauseBody(bodyPause{onWait: true, wait: wait})
}

// pauseForMessage pauses the body on the stack while wait, a wait for a message,
// goes on; nil where none is on the stack or the body cannot wait for one.
func (ctx *Context) pauseForMessage(wait bodyWait) error {
	if ctx.body == nil || !ctx.body.awaitsMessages {
		return nil
	}
	wait.onMessage = true
	return ctx.pauseBody(bodyPause{onWait: true, wait: wait})
}

// holdClock keeps the clock where it is while the behavior named runs: a wait on the
// clock under it, with no run to pause, is an error rather than an advance.
func (ctx *Context) holdClock(behavior string) func() {
	outer := ctx.clockHeldBy
	ctx.clockHeldBy = behavior
	restore := ctx.syncBoundary()
	return func() {
		restore()
		ctx.clockHeldBy = outer
	}
}

// driveClock reports the error where a wait on the clock, described by waits, cannot
// advance it because a behavior on the stack holds it; nil where the clock is free.
func (ctx *Context) driveClock(waits string) error {
	if ctx.clockHeldBy == "" {
		return nil
	}
	return fmt.Errorf("%w: %s waits for the clock (%s), which only a do behavior may",
		ErrStateBehaviorWaits, ctx.clockHeldBy, waits)
}

// pausedOnClock reports a token whose work waits on the clock through a flow it
// runs or an action it performs; the tokens parked there hold the wait, not this one.
func (t Token) pausedOnClock() bool {
	return t.body != nil && t.body.paused.onWait && !t.body.paused.wait.onMessage
}

// pausedOnMessage reports whether a token's paused body ultimately waits on a message.
func (t Token) pausedOnMessage() bool {
	return t.body != nil && t.body.paused.onWait && t.body.paused.wait.waitsForMessage(make(map[waitTarget]bool))
}

func (t Token) pausedOnChange() bool {
	return t.body != nil && t.body.paused.onWait && t.body.paused.wait.watchesChange(make(map[waitTarget]bool))
}

// resumable reports a token whose paused work would go on if resumed now: paused
// at a breakpoint, or on the clock for a wait that has ended.
func (t Token) resumable() bool {
	return t.body != nil && (!t.body.paused.onWait || !t.body.paused.wait.goesOn())
}

// pausedWaiter returns the executor the token's paused work waits on (the action it
// performs or the flow of a case it runs), nil for none; own is the token's executor.
func (t Token) pausedWaiter(own *ActionExecutor) clockWaiter {
	if t.body == nil {
		return nil
	}
	return t.body.paused.wait.waiter(own)
}

// hostedFlow returns the flow of a case the token's paused work runs, nil for none.
func (t Token) hostedFlow(own *ActionExecutor) *ActionExecutor {
	if t.body == nil {
		return nil
	}
	if w := t.body.paused.wait; w.held == nil && w.exec != nil && w.exec != own {
		return w.exec
	}
	return nil
}

// drivenByBody reports whether the token runs in a flow a body statement runs
// (runSubflow), whose steps that statement takes rather than Step.
func (t Token) drivenByBody() bool {
	for f := t.frame; f != nil; f = f.parent {
		if f.inBody {
			return true
		}
	}
	return false
}
