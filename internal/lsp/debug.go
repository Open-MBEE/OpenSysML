package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// The debug service runs the behavior a rendering draws and reports where it
// stands in that rendering's IDs, so a client can overlay the run on its drawing.
const (
	// MethodDebugStart renders the view, runs the target in a runtime built from
	// the workspace as it is now, and answers the initial snapshot.
	MethodDebugStart = "opensysml/debug/start"
	// MethodDebugStep advances the session by one step: one token move of an
	// action, or one event, change or do-round of a state machine.
	MethodDebugStep = "opensysml/debug/step"
	// MethodDebugContinue runs until the behavior completes, waits, reaches a
	// breakpoint or exhausts its budget, holding the clock where it is.
	MethodDebugContinue = "opensysml/debug/continue"
	// MethodDebugSend posts a signal to the behavior; the next step delivers it.
	MethodDebugSend = "opensysml/debug/send"
	// MethodDebugAdvance moves the runtime's clock forward, dispatching what
	// falls due on the way.
	MethodDebugAdvance = "opensysml/debug/advance"
	// MethodDebugBreakpoints replaces the session's breakpoints with the nodes
	// named, by render ID.
	MethodDebugBreakpoints = "opensysml/debug/breakpoints"
	// MethodDebugStop ends the session and releases its runtime.
	MethodDebugStop = "opensysml/debug/stop"
	// MethodDebugChanged notifies the client of a session a document change
	// moved to new render IDs or ended.
	MethodDebugChanged = "opensysml/debugChanged"
)

// debugProtocolVersion is the version of the snapshot shape; it moves when a
// field a client relies on changes meaning.
const debugProtocolVersion = 1

// Debug session states, as debugSnapshot.State reports them.
const (
	debugReady     = "ready"
	debugRunning   = "running"
	debugWaiting   = "waiting"
	debugSuspended = "suspended"
	debugCompleted = "completed"
	debugFailed    = "failed"
	debugEnded     = "ended"
)

// debugStartParams asks to run target, a state machine or action the view draws,
// performed by object when one is named (a part, item or occurrence definition or usage).
type debugStartParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	View         string                          `json:"view"`
	Target       string                          `json:"target"`
	Object       string                          `json:"object,omitempty"`
}

// debugSessionParams names the session a request drives.
type debugSessionParams struct {
	Session string `json:"session"`
}

// debugSendParams posts a signal; each argument is a SysML expression evaluated
// in the target's declaring scope, bound to the signal feature it is keyed by.
type debugSendParams struct {
	Session string            `json:"session"`
	Signal  string            `json:"signal"`
	Args    map[string]string `json:"args,omitempty"`
}

// debugAdvanceParams moves the clock forward by time.
type debugAdvanceParams struct {
	Session string  `json:"session"`
	Time    float64 `json:"time"`
}

// debugBreakpointsParams sets breakpoints on the nodes of the session's rendering
// named by ID, replacing the ones set before.
type debugBreakpointsParams struct {
	Session string   `json:"session"`
	NodeIDs []string `json:"nodeIds"`
}

// debugEdge names an edge of the session's rendering: its position among the
// render result's edges, with its endpoints for a client keyed by those.
type debugEdge struct {
	Index int    `json:"index"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// debugToken is a control token of an action flow at render node Node; Placed
// is false when it runs somewhere undrawn and Node is the innermost drawn node.
type debugToken struct {
	ID       int64       `json:"id"`
	Node     string      `json:"node"`
	Placed   bool        `json:"placed"`
	Via      *debugEdge  `json:"via,omitempty"`
	Awaiting []debugEdge `json:"awaiting,omitempty"`
	Waiting  string      `json:"waiting,omitempty"`
	Due      *float64    `json:"due,omitempty"`
}

// debugEvent is an event the behavior has yet to take: one queued in a state
// machine at the instant At, or one Pending on the runtime's message bus.
type debugEvent struct {
	Event   string  `json:"event"`
	At      float64 `json:"at"`
	Pending bool    `json:"pending,omitempty"`
}

// debugSnapshot is where a session stands, in the IDs of the render result of
// its view at Version. Every debug request answers one; debugChanged carries one.
type debugSnapshot struct {
	Protocol int    `json:"protocol"`
	Session  string `json:"session"`
	Kind     string `json:"kind"`
	View     string `json:"view"`
	Target   string `json:"target"`
	Object   string `json:"object,omitempty"`
	Root     string `json:"root"`
	Version  int    `json:"version"`
	// Revision counts the session's snapshots: a client keeps the highest it has
	// seen, as a notification taken earlier may reach it after a later answer.
	Revision int `json:"revision"`
	// State is ready, running, waiting, suspended, completed, failed or ended;
	// Reason says why for the last four.
	State  string  `json:"state"`
	Reason string  `json:"reason,omitempty"`
	Time   float64 `json:"time"`
	// Tokens and Taken describe an action; ActiveStates and Taken a machine.
	Tokens       []debugToken `json:"tokens"`
	ActiveStates []string     `json:"activeStates"`
	// Taken are the edges traversed since the previous snapshot: the transitions
	// a machine fired, in order, or the edges tokens arrived at new nodes by.
	Taken       []debugEdge  `json:"taken"`
	Queue       []debugEvent `json:"queue"`
	Breakpoints []string     `json:"breakpoints"`
	// PausedAt is the breakpoint node the last run stopped at, "" for none.
	PausedAt string `json:"pausedAt,omitempty"`
	// Notes are what the runtime noted during the last request.
	Notes []string `json:"notes"`
	// Results are a completed action's outputs.
	Results map[string]string `json:"results,omitempty"`
}

// Typed errors the debug requests answer.
var (
	// ErrDebugSession reports a request naming no live session.
	ErrDebugSession = errors.New("no such debug session")
	// ErrDebugTarget reports a target no debugger can run.
	ErrDebugTarget = errors.New("cannot debug target")
	// ErrDebugNode reports a breakpoint on a node the rendering does not draw.
	ErrDebugNode = errors.New("no such node in the session's rendering")
	// ErrDebugSignal reports a signal the behavior would not take.
	ErrDebugSignal = errors.New("signal refused")
	// ErrDebugEnded reports a request on a session that has ended.
	ErrDebugEnded = errors.New("debug session has ended")
	// ErrDebugTime reports an advance by something other than a finite duration.
	ErrDebugTime = errors.New("invalid duration")
)

// debugInvalid answers a request the client got wrong: the reply carries an
// InvalidParams code while the error still unwraps to its typed cause.
func debugInvalid(err error) error {
	return &debugRequestError{cause: err, wire: jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%s", err)}
}

type debugRequestError struct {
	cause error
	wire  *jsonrpc2.Error
}

func (e *debugRequestError) Error() string   { return e.cause.Error() }
func (e *debugRequestError) Unwrap() []error { return []error{e.cause, e.wire} }

// debugSession runs one behavior against the rendering drawing it.
type debugSession struct {
	id     string
	doc    string
	view   string
	kind   view.Kind
	target string
	object string
	// viewText is the declared view's text when the session started, "" for a
	// pseudo-view, which no declaration spells.
	viewText string

	rt        *runtime.Context
	runtime   *model.Runtime
	targetSym *symbols.Symbol
	objectSym *symbols.Symbol
	action    *runtime.ActionExecutor
	machine   *runtime.StateExecutor
	// reads are the declarations the run reads — the target's, the performer's,
	// what their text names and so on — as the runtime holds them.
	reads []model.Dependency
	// named are the declarations sends named besides those, by document and
	// name: what the run reads from too.
	named []dependencyKey

	// version is the document version the render IDs below belong to.
	version   int
	rendering *view.Rendering
	actions   *view.ActionLocator
	states    *view.StateLocator
	// nodes are the runtime nodes the drawn nodes stand for, by render ID.
	nodes map[string]debugNode
	// parents is the render node each render node is nested in.
	parents map[string]string
	// breakpoints are the runtime nodes stopped at, by render ID.
	breakpoints map[string]debugNode

	// noted and fired count the notes and edge traversals earlier snapshots reported.
	noted int
	fired int
	// revision is the number of snapshots taken, the last one's Revision.
	revision int
	// waiting explains an action parked on the clock; failure a run that
	// failed; ended why the session is over.
	waiting string
	failure string
	ended   string
	// paused is the render node of the breakpoint the last run stopped at, and
	// pausedName what the model calls it.
	paused     string
	pausedName string
}

// debugNode is a runtime node a render node draws: a vertex of a state graph,
// or an action node with the nested actions whose flows it runs in.
type debugNode struct {
	within []ast.Node
	node   ast.Node
}

// same reports whether both draw one runtime node: the same occurrence of it, for
// an action node a nested flow reuses.
func (n debugNode) same(other debugNode) bool {
	return n.node == other.node && slices.Equal(n.within, other.within)
}

// debugService holds the server's live sessions.
type debugService struct {
	mu       sync.Mutex
	next     int
	sessions map[string]*debugSession
}

func newDebugService() *debugService {
	return &debugService{sessions: make(map[string]*debugSession)}
}

// debugHandler answers the opensysml/debug/* requests.
func (s *Server) debugHandler(inner jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		var result *debugSnapshot
		var err error
		switch req.Method() {
		case MethodDebugStart:
			var params debugStartParams
			if perr := json.Unmarshal(req.Params(), &params); perr != nil {
				return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, perr))
			}
			result, err = s.DebugStart(&params)
		case MethodDebugStep, MethodDebugContinue, MethodDebugStop:
			var params debugSessionParams
			if perr := json.Unmarshal(req.Params(), &params); perr != nil {
				return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, perr))
			}
			switch req.Method() {
			case MethodDebugStep:
				result, err = s.DebugStep(&params)
			case MethodDebugContinue:
				result, err = s.DebugContinue(&params)
			default:
				result, err = s.DebugStop(&params)
			}
		case MethodDebugSend:
			var params debugSendParams
			if perr := json.Unmarshal(req.Params(), &params); perr != nil {
				return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, perr))
			}
			result, err = s.DebugSend(&params)
		case MethodDebugAdvance:
			var params debugAdvanceParams
			if perr := json.Unmarshal(req.Params(), &params); perr != nil {
				return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, perr))
			}
			result, err = s.DebugAdvance(&params)
		case MethodDebugBreakpoints:
			var params debugBreakpointsParams
			if perr := json.Unmarshal(req.Params(), &params); perr != nil {
				return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, perr))
			}
			result, err = s.DebugBreakpoints(&params)
		default:
			return inner(ctx, reply, req)
		}
		if err != nil {
			return reply(ctx, nil, err)
		}
		return reply(ctx, result, nil)
	}
}

// DebugStart answers opensysml/debug/start.
func (s *Server) DebugStart(params *debugStartParams) (*debugSnapshot, error) {
	sess, err := s.debugPrepare(params)
	if err != nil {
		return nil, err
	}
	return s.debugRegister(sess)
}

// debugPrepare builds the session params asks for, run and located in the
// documents as one reading of the workspace has them, but not yet registered.
func (s *Server) debugPrepare(params *debugStartParams) (*debugSession, error) {
	name := uriToName(params.TextDocument.URI)
	if strings.TrimSpace(params.Target) == "" {
		return nil, debugInvalid(fmt.Errorf("%w: no target named", ErrDebugTarget))
	}
	// The rendering, the runtime and the view's text are read together, so the
	// IDs answered and the behavior run are of one and the same documents.
	var (
		rendering *view.Rendering
		doc       *model.Document
		rt        *model.Runtime
		viewText  string
	)
	if err := s.ws.Read(func(r *model.Reading) (err error) {
		if rendering, doc, err = r.RenderView(name, params.View); err != nil {
			return err
		}
		if rendering.Kind != view.KindState && rendering.Kind != view.KindAction {
			return debugInvalid(fmt.Errorf("%w: %s renders a %s, which no debugger drives", ErrDebugTarget, params.View, rendering.Kind))
		}
		if rt, err = r.NewRuntime(); err != nil {
			return err
		}
		viewText = r.DeclarationText(r.DeclaredView(name, params.View))
		return nil
	}); err != nil {
		return nil, err
	}
	target := rt.Declared(name, params.Target)
	if target == nil {
		return nil, debugInvalid(fmt.Errorf("%w: %s declares no %s", ErrDebugTarget, name, params.Target))
	}
	if err := debugTargetKind(rendering.Kind, target); err != nil {
		return nil, debugInvalid(err)
	}
	ctx := runtime.NewContext(rt.Model(), runtime.DefaultBudgets().MaxSteps)
	var objectSym *symbols.Symbol
	var performer *runtime.Instance
	if params.Object != "" {
		objectSym = rt.Declared(name, params.Object)
		if objectSym == nil {
			return nil, debugInvalid(fmt.Errorf("%w: %s declares no %s to perform it", ErrDebugTarget, name, params.Object))
		}
		if !debugPerformerKind(objectSym.Kind) {
			return nil, debugInvalid(fmt.Errorf("%w: %s is a %s, not an object that could perform %s", ErrDebugTarget, params.Object, objectSym.Notation(), params.Target))
		}
		var err error
		if performer, err = ctx.Instantiate(objectSym); err != nil {
			return nil, fmt.Errorf("%w: instantiate %s: %w", ErrDebugTarget, params.Object, err)
		}
	}
	sess := &debugSession{
		doc:       name,
		view:      params.View,
		kind:      rendering.Kind,
		target:    rt.FQN(target),
		rt:        ctx,
		runtime:   rt,
		targetSym: target,
		objectSym: objectSym,
		viewText:  viewText,
	}
	if objectSym != nil {
		sess.object = rt.FQN(objectSym)
	}
	if err := sess.attach(ctx, target, performer); err != nil {
		return nil, err
	}
	sess.reads = rt.Dependencies(target, objectSym)
	if err := sess.locate(rendering, target, doc.Version); err != nil {
		sess.release()
		return nil, err
	}
	return sess, nil
}

// debugRegister gives a prepared session its ID and starts answering for it.
func (s *Server) debugRegister(sess *debugSession) (*debugSnapshot, error) {
	s.debug.mu.Lock()
	defer s.debug.mu.Unlock()
	// An edit since the session's reading is reconciled under the lock edits
	// reconcile under, so none passes the session by unseen.
	if s.ws.Generation() != sess.runtime.Generation() {
		if _, moved := s.reconcile(sess); moved && sess.ended != "" {
			sess.release()
			return nil, debugInvalid(fmt.Errorf("%w: %s", ErrDebugTarget, sess.ended))
		}
	}
	s.debug.next++
	sess.id = "debug-" + strconv.Itoa(s.debug.next)
	s.debug.sessions[sess.id] = sess
	return sess.snapshot(), nil
}

// attach gives the session the executor of target: the one performer already
// runs when it runs target, else a fresh one, as the REPL attaches a behavior.
func (sess *debugSession) attach(ctx *runtime.Context, target *symbols.Symbol, performer *runtime.Instance) error {
	var running []*runtime.ObjectBehavior
	if performer != nil {
		switch sess.kind {
		case view.KindState:
			running = performer.ExhibitedStatesOf(target)
		case view.KindAction:
			running = performer.PerformedActionsOf(target)
		}
	}
	if len(running) > 1 {
		usages := make([]string, 0, len(running))
		for _, b := range running {
			usages = append(usages, b.Describe())
		}
		return debugInvalid(fmt.Errorf("%w: %s runs %s %d times, so name the usage: %s",
			ErrDebugTarget, sess.object, sess.target, len(running), strings.Join(usages, "; ")))
	}
	if len(running) == 1 {
		sess.machine, sess.action = running[0].State, running[0].Action
		return nil
	}
	var err error
	switch sess.kind {
	case view.KindState:
		sess.machine, err = ctx.CreateStateExecutorFor(target, performer)
	case view.KindAction:
		sess.action, err = ctx.CreateActionExecutorFor(target, performer)
	}
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrDebugTarget, sess.target, err)
	}
	return nil
}

// debugTargetKind checks target declares what a rendering of kind draws.
func debugTargetKind(kind view.Kind, target *symbols.Symbol) error {
	switch kind {
	case view.KindState:
		if target.Kind == symbols.SymbolStateDef || target.Kind == symbols.SymbolStateUsage {
			return nil
		}
		return fmt.Errorf("%w: %s is a %s, not a state machine", ErrDebugTarget, target.Name, target.Notation())
	case view.KindAction:
		if target.Kind == symbols.SymbolActionDef || target.Kind == symbols.SymbolActionUsage {
			return nil
		}
		return fmt.Errorf("%w: %s is a %s, not an action", ErrDebugTarget, target.Name, target.Notation())
	}
	return fmt.Errorf("%w: a %s rendering draws no behavior", ErrDebugTarget, kind)
}

// debugPerformerKind reports whether a declaration of kind is an object — a part,
// item or occurrence definition or usage — and so can perform a behavior.
func debugPerformerKind(kind symbols.SymbolKind) bool {
	switch kind {
	case symbols.SymbolPartDef, symbols.SymbolPartUsage,
		symbols.SymbolItemDef, symbols.SymbolItemUsage,
		symbols.SymbolOccurrenceDef, symbols.SymbolOccurrenceUsage,
		symbols.SymbolIndividualDef, symbols.SymbolIndividualUsage:
		return true
	}
	return false
}

// locate binds the session to rendering, in which drawn declares the target at
// version, keeping the breakpoints on runtime nodes still drawn.
func (sess *debugSession) locate(rendering *view.Rendering, drawn *symbols.Symbol, version int) error {
	nodes := make(map[string]debugNode)
	switch sess.kind {
	case view.KindState:
		loc, err := view.LocateStates(rendering, drawn, sess.targetSym, sess.machine.Graph())
		if err != nil {
			return err
		}
		graph := sess.machine.Graph()
		for _, state := range graph.States {
			if id, ok := loc.Node(state); ok {
				nodes[id] = debugNode{node: state}
			}
		}
		for _, pseudo := range graph.Pseudostates {
			if id, ok := loc.Node(pseudo); ok {
				nodes[id] = debugNode{node: pseudo}
			}
		}
		sess.states, sess.actions = loc, nil
	case view.KindAction:
		loc, err := view.LocateActions(rendering, drawn, sess.targetSym, sess.action.Graph())
		if err != nil {
			return err
		}
		var walk func(graph *lower.ActionGraph, within []ast.Node)
		walk = func(graph *lower.ActionGraph, within []ast.Node) {
			for _, node := range graph.Nodes {
				if id, ok := loc.Node(within, node); ok {
					nodes[id] = debugNode{within: within, node: node}
				}
				if sub := graph.Subflows[node]; sub != nil && sub.Graph != nil && sub.Err == nil {
					walk(sub.Graph, append(append([]ast.Node(nil), within...), node))
				}
			}
		}
		walk(sess.action.Graph(), nil)
		sess.actions, sess.states = loc, nil
	}
	sess.rendering, sess.version, sess.nodes = rendering, version, nodes
	sess.parents = make(map[string]string)
	var nest func(nodes []*view.Node, parent string)
	nest = func(nodes []*view.Node, parent string) {
		for _, n := range nodes {
			if parent != "" {
				sess.parents[n.ID] = parent
			}
			nest(n.Children, n.ID)
		}
	}
	nest(rendering.Roots, "")
	kept := make(map[string]debugNode, len(sess.breakpoints))
	for id, bp := range nodes {
		for _, old := range sess.breakpoints {
			if old.same(bp) {
				kept[id] = bp
			}
		}
	}
	sess.breakpoints = kept
	sess.applyBreakpoints()
	// A pause stands through a redraw; its node is named in the fresh IDs.
	sess.paused, sess.pausedName = "", ""
	sess.pause()
	return nil
}

// applyBreakpoints sets the executor's breakpoints to the session's; a pause
// already reached at one kept stands, so the next run resumes past it.
func (sess *debugSession) applyBreakpoints() {
	if sess.action != nil {
		bps := make([]runtime.NodeBreakpoint, 0, len(sess.breakpoints))
		for _, bp := range sess.breakpoints {
			bps = append(bps, runtime.NodeBreakpoint{Within: bp.within, Node: bp.node})
		}
		sess.action.ReplaceBreakpointsAt(bps)
		return
	}
	sess.machine.ClearBreakpoints()
	for _, bp := range sess.breakpoints {
		sess.machine.SetBreakpointAt(bp.node)
	}
}

// halted reports the session's executor paused at a breakpoint.
func (sess *debugSession) halted() bool {
	if sess.action != nil {
		return sess.action.PausedAt() != ""
	}
	return sess.machine.PausedAt() != nil
}

// release lets the session's executor go.
func (sess *debugSession) release() {
	if sess.action != nil {
		sess.action.Release()
	}
	if sess.machine != nil {
		sess.machine.Release()
	}
}

// session is the live session id names, locked; the caller unlocks.
func (s *Server) session(id string) (*debugSession, error) {
	s.debug.mu.Lock()
	sess, ok := s.debug.sessions[id]
	if !ok {
		s.debug.mu.Unlock()
		return nil, debugInvalid(fmt.Errorf("%w: %q", ErrDebugSession, id))
	}
	return sess, nil
}

// live is a session a request may drive: one neither ended nor failed.
func (sess *debugSession) live() error {
	if sess.ended != "" {
		return fmt.Errorf("%w: %s", ErrDebugEnded, sess.ended)
	}
	return nil
}

// DebugStep answers opensysml/debug/step.
func (s *Server) DebugStep(params *debugSessionParams) (*debugSnapshot, error) {
	sess, err := s.session(params.Session)
	if err != nil {
		return nil, err
	}
	defer s.debug.mu.Unlock()
	if err := sess.live(); err != nil {
		return nil, err
	}
	sess.resume()
	switch sess.kind {
	case view.KindAction:
		if sess.action.State() == runtime.StateCompleted {
			break
		}
		err := sess.action.StepToBreakpoint()
		if errors.Is(err, runtime.ErrNothingDue) {
			sess.waiting = debugClockWait(sess.action)
		} else if err != nil {
			sess.failure = err.Error()
		}
	case view.KindState:
		if err := sess.stepMachine(); err != nil {
			sess.failure = err.Error()
		}
	}
	sess.pause()
	return sess.snapshot(), nil
}

// resume forgets what the last run ended on before the next one, releasing the
// executor from the breakpoint it paused at.
func (sess *debugSession) resume() {
	sess.waiting, sess.failure = "", ""
	sess.paused, sess.pausedName = "", ""
	if sess.machine != nil && sess.machine.PausedAt() != nil {
		sess.machine.Resume()
	}
	if sess.action != nil && sess.action.PausedAt() != "" {
		sess.action.Resume()
	}
}

// pause records the breakpoint node the run just done stopped at, if any.
func (sess *debugSession) pause() {
	if sess.machine != nil {
		if vertex := sess.machine.PausedAt(); vertex != nil {
			sess.paused, _ = sess.states.Node(vertex)
			sess.pausedName = runtime.StateVertexName(vertex)
		}
		return
	}
	if bp, ok := sess.action.PausedBreakpoint(); ok {
		sess.paused, _ = sess.actions.Node(bp.Within, bp.Node)
		sess.pausedName = sess.action.PausedAt()
	}
}

// clockWaiter is an executor with waits on the clock.
type clockWaiter interface {
	NextWait() (float64, bool)
}

// debugClockWait says what a behavior parked on the clock waits for.
func debugClockWait(exec clockWaiter) string {
	if due, ok := exec.NextWait(); ok {
		return "waits on the clock until t=" + semantics.FormatReal(due)
	}
	return "waits on the clock"
}

// stepMachine advances a state machine one step as the REPL does: a change
// condition that fires, else the next event, else a round of do behavior.
func (sess *debugSession) stepMachine() error {
	exec := sess.machine
	if exec.HasPendingWork() || exec.WatchesChangeCondition() {
		exec.Resume()
	}
	if exec.State() != runtime.StateRunning {
		return nil
	}
	fired, err := exec.PollChangeEvents()
	if err != nil {
		return fmt.Errorf("change condition failed: %w", err)
	}
	if fired {
		return nil
	}
	if exec.EventQueue().Len() > 0 || exec.HasPendingSignal() {
		if err := exec.ProcessNextEvent(); err != nil {
			return fmt.Errorf("event processing failed: %w", err)
		}
		return nil
	}
	if exec.HasPendingDoWork() {
		if _, err := exec.RunDoRound(); err != nil {
			return fmt.Errorf("do behavior failed: %w", err)
		}
		return nil
	}
	exec.Suspend()
	return nil
}

// DebugContinue answers opensysml/debug/continue.
func (s *Server) DebugContinue(params *debugSessionParams) (*debugSnapshot, error) {
	sess, err := s.session(params.Session)
	if err != nil {
		return nil, err
	}
	defer s.debug.mu.Unlock()
	if err := sess.live(); err != nil {
		return nil, err
	}
	sess.resume()
	switch sess.kind {
	case view.KindAction:
		if sess.action.State() == runtime.StateCompleted {
			break
		}
		if err := sess.action.RunToQuiescence(); err != nil {
			sess.failure = err.Error()
		} else if sess.action.State() == runtime.StateWaiting && sess.action.PausedAt() == "" {
			if _, due := sess.action.NextWait(); due && !sess.action.HasPendingSignal() {
				sess.waiting = debugClockWait(sess.action)
			}
		}
	case view.KindState:
		sess.continueMachine()
	}
	sess.pause()
	return sess.snapshot(), nil
}

// continueMachine runs a state machine to quiescence at the current instant or
// to a breakpoint, holding the clock: an event due later waits for an advance.
func (sess *debugSession) continueMachine() {
	exec := sess.machine
	if exec.State() == runtime.StateCompleted {
		return
	}
	if err := exec.RunToQuiescence(); err != nil {
		sess.failure = err.Error()
	}
}

// DebugSend answers opensysml/debug/send.
func (s *Server) DebugSend(params *debugSendParams) (*debugSnapshot, error) {
	sess, err := s.session(params.Session)
	if err != nil {
		return nil, err
	}
	defer s.debug.mu.Unlock()
	if err := sess.live(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Signal) == "" {
		return nil, debugInvalid(fmt.Errorf("%w: no signal named", ErrDebugSignal))
	}
	send, err := sess.parseSend(params.Signal, params.Args)
	if err != nil {
		return nil, debugInvalid(fmt.Errorf("%w: %w", ErrDebugSignal, err))
	}
	if change := s.debugReadsToo(sess, send.named(sess)); change != "" {
		delete(s.debug.sessions, sess.id)
		sess.ended = change
		sess.release()
		return sess.snapshot(), nil
	}
	msg, err := sess.signalMessage(send)
	if err != nil {
		return nil, debugInvalid(fmt.Errorf("%w: %w", ErrDebugSignal, err))
	}
	var accepted bool
	switch sess.kind {
	case view.KindAction:
		accepted, err = sess.action.AcceptsMessage(msg)
	case view.KindState:
		accepted, err = sess.machine.AcceptsMessage(msg)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDebugSignal, err)
	}
	if !accepted {
		return nil, debugInvalid(fmt.Errorf("%w: %s accepts no signal %s now", ErrDebugSignal, sess.target, msg.SignalType))
	}
	sess.rt.PostMessage(msg)
	return sess.snapshot(), nil
}

// debugSend is a send as parsed: the signal named, its definition when one is
// declared, and its arguments' expressions by name, sorted.
type debugSend struct {
	signal string
	def    *symbols.Symbol
	args   []debugArgument
}

type debugArgument struct {
	name string
	expr ast.Node
}

// parseSend reads the signal and arguments of a send, evaluating nothing yet.
func (sess *debugSession) parseSend(signal string, args map[string]string) (*debugSend, error) {
	send := &debugSend{signal: signal, def: sess.signalDefinition(signal)}
	if send.def == nil && len(args) > 0 {
		return nil, fmt.Errorf("no signal definition %s is declared, so it cannot carry arguments", signal)
	}
	if send.def != nil && !runtime.IsSignalDefinition(send.def) {
		return nil, fmt.Errorf("%s is a %s, not a signal definition", signal, send.def.Notation())
	}
	names := make([]string, 0, len(args))
	for name := range args {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		expr, err := parseDebugExpression(args[name])
		if err != nil {
			return nil, fmt.Errorf("argument %s: %w", name, err)
		}
		send.args = append(send.args, debugArgument{name: name, expr: expr})
	}
	return send, nil
}

// named is the declarations the send reads: the signal's definition and what
// its arguments name, as the session's runtime resolves them.
func (send *debugSend) named(sess *debugSession) []*symbols.Symbol {
	var out []*symbols.Symbol
	if send.def != nil {
		out = append(out, send.def)
	}
	for _, arg := range send.args {
		out = append(out, sess.runtime.Referenced(sess.targetSym.OwnerScope, arg.expr)...)
	}
	return out
}

// signalMessage builds the message a send posts: typed by the signal definition
// named, or by name alone when none is declared.
func (sess *debugSession) signalMessage(send *debugSend) (runtime.Message, error) {
	if send.def == nil {
		return runtime.NamedSignalMessage(send.signal, sess.performer()), nil
	}
	bound := make(map[string]runtime.Value, len(send.args))
	for _, arg := range send.args {
		value, err := sess.rt.EvalWithScope(arg.expr, sess.targetSym.OwnerScope)
		if err != nil {
			return runtime.Message{}, fmt.Errorf("argument %s: %w", arg.name, err)
		}
		bound[arg.name] = value
	}
	return sess.rt.SignalMessage(send.def, bound, sess.performer())
}

// debugReadsToo adds to what the session reads the declarations reachable from
// named, checking the workspace still holds them as the runtime read them; the
// change that says it does not ends the session, "" when it does.
func (s *Server) debugReadsToo(sess *debugSession, named []*symbols.Symbol) string {
	var roots []*symbols.Symbol
	var keys []dependencyKey
	for _, sym := range named {
		if dep, ok := sess.runtime.DeclarationOf(sym); ok && !slices.Contains(sess.named, keyOf(dep)) && !slices.Contains(keys, keyOf(dep)) {
			roots = append(roots, sym)
			keys = append(keys, keyOf(dep))
		}
	}
	if len(roots) == 0 {
		return ""
	}
	fresh := sess.runtime.Dependencies(roots...)
	known := make(map[dependencyKey]bool, len(sess.reads))
	for _, dep := range sess.reads {
		known[keyOf(dep)] = true
	}
	if slices.IndexFunc(fresh, func(dep model.Dependency) bool { return !known[keyOf(dep)] }) < 0 {
		sess.named = append(sess.named, keys...)
		return ""
	}
	var change string
	_ = s.ws.Read(func(r *model.Reading) error {
		roots, gone := sess.namedIn(r, keys)
		if gone != "" {
			change = gone
			return nil
		}
		change = dependencyChange(r, sess.target, fresh, r.Dependencies(roots...))
		return nil
	})
	if change != "" {
		return change
	}
	for _, dep := range fresh {
		if !known[keyOf(dep)] {
			sess.reads = append(sess.reads, dep)
		}
	}
	sess.named = append(sess.named, keys...)
	return ""
}

// namedIn is the declarations keys name as r reads them, or why one is gone.
func (sess *debugSession) namedIn(r *model.Reading, keys []dependencyKey) ([]*symbols.Symbol, string) {
	roots := make([]*symbols.Symbol, 0, len(keys))
	for _, key := range keys {
		sym := r.Declared(key.doc, key.fqn)
		if sym == nil {
			return nil, fmt.Sprintf("%s is no longer declared", key.fqn)
		}
		roots = append(roots, sym)
	}
	return roots, ""
}

// signalDefinition is the definition signal names, by qualified name anywhere
// or by name among the target's document, nil when none is declared.
func (sess *debugSession) signalDefinition(signal string) *symbols.Symbol {
	if sym := sess.runtime.Declared(sess.doc, signal); sym != nil {
		return sym
	}
	for _, sym := range sess.runtime.Lookup(signal) {
		return sym
	}
	return nil
}

// performer is the object performing the session's behavior, nil for none.
func (sess *debugSession) performer() *runtime.Instance {
	if sess.action != nil {
		return sess.action.Performer()
	}
	return sess.machine.Performer()
}

// debugExprPrefix wraps an expression as a usage value so the parser reads it.
const debugExprPrefix = "attribute expr = "

// parseDebugExpression parses one SysML expression written on its own.
func parseDebugExpression(text string) (ast.Node, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("empty expression")
	}
	p := parser.New(source.New("argument", []byte(debugExprPrefix+text+";")))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		return nil, fmt.Errorf("%q: %s", text, p.Diagnostics[0].Message)
	}
	if len(root.Members) == 1 {
		member := root.Members[0]
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		if usage, ok := member.(*ast.Usage); ok && usage.Value != nil {
			return usage.Value, nil
		}
	}
	return nil, fmt.Errorf("%q is not an expression", text)
}

// DebugAdvance answers opensysml/debug/advance.
func (s *Server) DebugAdvance(params *debugAdvanceParams) (*debugSnapshot, error) {
	if params.Time < 0 || math.IsNaN(params.Time) || math.IsInf(params.Time, 0) {
		return nil, debugInvalid(fmt.Errorf("%w: time must be a finite duration of at least 0, not %v", ErrDebugTime, params.Time))
	}
	sess, err := s.session(params.Session)
	if err != nil {
		return nil, err
	}
	defer s.debug.mu.Unlock()
	if err := sess.live(); err != nil {
		return nil, err
	}
	sess.resume()
	if _, err := sess.rt.AdvanceUntil(params.Time, sess.halted); err != nil {
		sess.failure = err.Error()
	}
	sess.pause()
	return sess.snapshot(), nil
}

// DebugBreakpoints answers opensysml/debug/breakpoints.
func (s *Server) DebugBreakpoints(params *debugBreakpointsParams) (*debugSnapshot, error) {
	sess, err := s.session(params.Session)
	if err != nil {
		return nil, err
	}
	defer s.debug.mu.Unlock()
	if err := sess.live(); err != nil {
		return nil, err
	}
	set := make(map[string]debugNode, len(params.NodeIDs))
	for _, id := range params.NodeIDs {
		node, ok := sess.nodes[id]
		if !ok {
			return nil, debugInvalid(fmt.Errorf("%w: %q is no node of %s at version %d that runs", ErrDebugNode, id, sess.view, sess.version))
		}
		set[id] = node
	}
	sess.breakpoints = set
	sess.applyBreakpoints()
	return sess.snapshot(), nil
}

// DebugStop answers opensysml/debug/stop.
func (s *Server) DebugStop(params *debugSessionParams) (*debugSnapshot, error) {
	sess, err := s.session(params.Session)
	if err != nil {
		return nil, err
	}
	defer s.debug.mu.Unlock()
	delete(s.debug.sessions, sess.id)
	if sess.ended == "" {
		sess.ended = "stopped"
	}
	sess.release()
	return sess.snapshot(), nil
}

// snapshot is where the session stands now.
func (sess *debugSession) snapshot() *debugSnapshot {
	snap := &debugSnapshot{
		Protocol:     debugProtocolVersion,
		Session:      sess.id,
		Kind:         string(sess.kind),
		View:         sess.view,
		Target:       sess.target,
		Object:       sess.object,
		Version:      sess.version,
		Time:         sess.rt.Clock().Now(),
		Tokens:       []debugToken{},
		ActiveStates: []string{},
		Taken:        []debugEdge{},
		Queue:        []debugEvent{},
		Breakpoints:  sess.breakpointIDs(),
		PausedAt:     sess.paused,
		Notes:        []string{},
	}
	sess.revision++
	snap.Revision = sess.revision
	var notes []runtime.RunNote
	switch sess.kind {
	case view.KindAction:
		snap.Root = sess.actions.Root()
		sess.actionSnapshot(snap)
		notes, sess.noted = sess.action.NotesSince(sess.noted), sess.action.NoteCount()
	case view.KindState:
		snap.Root = sess.states.Root()
		sess.machineSnapshot(snap)
		notes, sess.noted = sess.machine.NotesSince(sess.noted), sess.machine.NoteCount()
	}
	for _, note := range notes {
		snap.Notes = append(snap.Notes, note.String())
	}
	snap.State, snap.Reason = sess.status()
	return snap
}

// status is the session's state and the reason for it.
func (sess *debugSession) status() (string, string) {
	switch {
	case sess.ended != "":
		return debugEnded, sess.ended
	case sess.failure != "":
		return debugFailed, sess.failure
	case sess.waiting != "":
		return debugWaiting, sess.waiting
	}
	var state runtime.ExecutionState
	if sess.action != nil {
		state = sess.action.State()
	} else {
		state = sess.machine.State()
	}
	switch state {
	case runtime.StateReady:
		return debugReady, ""
	case runtime.StateRunning:
		return debugRunning, ""
	case runtime.StateWaiting:
		return debugWaiting, sess.waitReason()
	case runtime.StateSuspended:
		if sess.machineWaitsOnClock() {
			return debugWaiting, debugClockWait(sess.machine)
		}
		return debugSuspended, sess.suspendReason()
	case runtime.StateCompleted:
		return debugCompleted, ""
	}
	return strings.ToLower(state.String()), ""
}

// waitReason says what a waiting behavior waits for.
func (sess *debugSession) waitReason() string {
	if sess.action != nil {
		if sess.action.PausedAt() != "" {
			return ""
		}
		var waits []string
		for _, tok := range sess.action.Tokens() {
			if w := debugWait(tok); w != "" {
				waits = append(waits, w)
			}
		}
		return strings.Join(waits, "; ")
	}
	return sess.machine.SuspendReason()
}

// machineWaitsOnClock is a quiescent machine whose only pending work is an
// event due later, which an advance of the clock delivers.
func (sess *debugSession) machineWaitsOnClock() bool {
	exec := sess.machine
	if exec == nil || sess.paused != "" || exec.SuspendReason() != "" || exec.HasPendingSignal() {
		return false
	}
	_, due := exec.NextWait()
	return due
}

// suspendReason says why a behavior is suspended: a breakpoint or quiescence.
func (sess *debugSession) suspendReason() string {
	if sess.paused != "" {
		return "paused at breakpoint " + sess.pausedName
	}
	if sess.action != nil {
		return ""
	}
	if reason := sess.machine.SuspendReason(); reason != "" {
		return reason
	}
	return "the machine is quiescent"
}

// breakpointIDs lists the breakpoints by render ID, in order.
func (sess *debugSession) breakpointIDs() []string {
	ids := make([]string, 0, len(sess.breakpoints))
	for id := range sess.breakpoints {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// actionSnapshot fills in the tokens of an action and the edges they took.
func (sess *debugSession) actionSnapshot(snap *debugSnapshot) {
	exec := sess.action
	for _, tr := range exec.TraversalsSince(sess.fired) {
		if edge, ok := sess.actionEdge(tr.Within, tr.Edge); ok {
			snap.Taken = append(snap.Taken, edge)
		}
	}
	sess.fired = exec.TraversalCount()
	for _, tok := range exec.Tokens() {
		within := tok.Within()
		id, drawn := sess.actions.Node(within, tok.Location)
		t := debugToken{ID: tok.ID, Node: id, Placed: drawn, Waiting: debugWait(tok)}
		if tok.Wait != nil && tok.Wait.Timed {
			due := tok.Wait.Due
			t.Due = &due
		}
		if tok.Via.Source != nil || tok.Via.Target != nil {
			if edge, ok := sess.actionEdge(within, tok.Via); ok {
				t.Via = &edge
			}
		}
		for _, await := range exec.Awaiting(tok) {
			if edge, ok := sess.actionEdge(within, await); ok {
				t.Awaiting = append(t.Awaiting, edge)
			}
		}
		snap.Tokens = append(snap.Tokens, t)
	}
	for _, msg := range sess.rt.PendingMessages() {
		snap.Queue = append(snap.Queue, debugEvent{Event: debugSignalText(msg), At: snap.Time, Pending: true})
	}
	if exec.State() == runtime.StateCompleted {
		results := exec.Results()
		if len(results) > 0 {
			snap.Results = make(map[string]string, len(results))
			for name, value := range results {
				snap.Results[name] = runtime.FormatValue(value)
			}
		}
	}
}

// enclosing is the render node id with the nodes it is nested in below the
// root, outermost first: the states and regions an active vertex makes active.
func (sess *debugSession) enclosing(id string) []string {
	var chain []string
	for cur := id; cur != "" && cur != sess.root(); cur = sess.parents[cur] {
		chain = append(chain, cur)
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// root is the render node drawing the behavior itself.
func (sess *debugSession) root() string {
	if sess.states != nil {
		return sess.states.Root()
	}
	return sess.actions.Root()
}

// actionEdge is the render edge drawing edge in the flow of within.
func (sess *debugSession) actionEdge(within []ast.Node, edge lower.ActionEdge) (debugEdge, bool) {
	index, ok := sess.actions.Edge(within, edge)
	if !ok {
		return debugEdge{}, false
	}
	return sess.edgeAt(index), true
}

// edgeAt is the render edge at index in the rendering's edges.
func (sess *debugSession) edgeAt(index int) debugEdge {
	edge := sess.rendering.Edges[index]
	return debugEdge{Index: index, From: edge.From, To: edge.To}
}

// debugWait says what a parked token waits for, "" for a token that runs.
func debugWait(tok runtime.Token) string {
	w := tok.Wait
	if w == nil {
		return ""
	}
	switch {
	case w.Timed:
		return "until t=" + semantics.FormatReal(w.Due)
	case w.Trigger != "":
		return w.Trigger
	case w.SignalType != "" && w.ViaPort != "":
		return w.SignalType + " via " + w.ViaPort
	case w.SignalType != "":
		return w.SignalType
	case w.ViaPort != "":
		return "any signal via " + w.ViaPort
	}
	return "any signal"
}

// machineSnapshot fills in the active configuration of a machine, the
// transitions it took since the last snapshot, and the events it has yet to take.
func (sess *debugSession) machineSnapshot(snap *debugSnapshot) {
	exec := sess.machine
	seen := make(map[string]bool)
	activate := func(vertex ast.Node) {
		id, ok := sess.states.Node(vertex)
		if !ok {
			return
		}
		for _, id := range sess.enclosing(id) {
			if !seen[id] {
				seen[id] = true
				snap.ActiveStates = append(snap.ActiveStates, id)
			}
		}
	}
	for _, state := range exec.ActiveStates() {
		activate(state)
	}
	if current := exec.CurrentState(); current != nil {
		activate(current)
	}
	for _, f := range exec.FiredSince(sess.fired) {
		var index int
		var ok bool
		if f.Source == nil {
			index, ok = sess.states.EntryTransition(f.Decl, f.Owner, f.Target)
		} else {
			index, ok = sess.states.Transition(f.Decl, f.Source, f.Target)
		}
		if ok {
			snap.Taken = append(snap.Taken, sess.edgeAt(index))
		}
	}
	sess.fired = exec.FiredCount()
	for _, event := range exec.EventQueue().Events() {
		snap.Queue = append(snap.Queue, debugEvent{Event: debugEventText(event), At: event.Timestamp})
	}
	for _, msg := range sess.rt.PendingMessages() {
		snap.Queue = append(snap.Queue, debugEvent{Event: debugSignalText(msg), At: snap.Time, Pending: true})
	}
}

// debugEventText names a queued event by what it carries.
func debugEventText(event runtime.Event) string {
	switch payload := event.Payload.(type) {
	case runtime.Message:
		return debugSignalText(payload)
	case runtime.Call:
		return payload.Operation + "()"
	}
	return event.Type.String()
}

// debugSignalText writes a message as signal(arg=value, …).
func debugSignalText(msg runtime.Message) string {
	if len(msg.Payload) == 0 {
		if msg.Value != nil {
			return msg.SignalType + "(" + runtime.FormatValue(*msg.Value) + ")"
		}
		return msg.SignalType
	}
	names := make([]string, 0, len(msg.Payload))
	for name := range msg.Payload {
		names = append(names, name)
	}
	sort.Strings(names)
	args := make([]string, 0, len(names))
	for _, name := range names {
		args = append(args, name+"="+runtime.FormatValue(msg.Payload[name]))
	}
	return msg.SignalType + "(" + strings.Join(args, ", ") + ")"
}

// debugDocumentsChanged ends the sessions whose behavior, performer or view an
// edit rewrote, moves the rest to the fresh IDs, and announces each change.
func (s *Server) debugDocumentsChanged(ctx context.Context) {
	s.debug.mu.Lock()
	var changed []*debugSnapshot
	for id, sess := range s.debug.sessions {
		snap, moved := s.reconcile(sess)
		if !moved {
			continue
		}
		if sess.ended != "" {
			delete(s.debug.sessions, id)
			sess.release()
		}
		changed = append(changed, snap)
	}
	s.debug.mu.Unlock()
	s.notifyDebugChanged(ctx, changed)
}

// reconcile rebinds sess to the workspace as it is now, reporting whether the
// session moved: to new render IDs, or to its end.
func (s *Server) reconcile(sess *debugSession) (*debugSnapshot, bool) {
	var moved bool
	// One reading, so the declarations compared and the rendering located in
	// are of the same documents.
	_ = s.ws.Read(func(r *model.Reading) error {
		moved = sess.rebind(r)
		return nil
	})
	if !moved {
		return nil, false
	}
	return sess.snapshot(), true
}

// rebind binds sess to the documents as r reads them, reporting whether the
// session moved: to new render IDs, or to its end.
func (sess *debugSession) rebind(r *model.Reading) bool {
	end := func(reason string) bool {
		sess.ended = reason
		return true
	}
	if r.Document(sess.doc) == nil {
		return end(fmt.Sprintf("%s was closed", sess.doc))
	}
	target := r.Declared(sess.doc, sess.target)
	if target == nil {
		return end(fmt.Sprintf("%s is no longer declared", sess.target))
	}
	var object *symbols.Symbol
	if sess.objectSym != nil {
		if object = r.Declared(sess.doc, sess.object); object == nil {
			return end(fmt.Sprintf("%s is no longer declared", sess.object))
		}
	}
	roots, gone := sess.namedIn(r, sess.named)
	if gone != "" {
		return end(gone)
	}
	roots = append(roots, target, object)
	if change := dependencyChange(r, sess.target, sess.reads, r.Dependencies(roots...)); change != "" {
		return end(change)
	}
	rendering, doc, err := r.RenderView(sess.doc, sess.view)
	if err != nil {
		return end(fmt.Sprintf("%s no longer renders: %v", sess.view, err))
	}
	if rendering.Kind != sess.kind {
		return end(fmt.Sprintf("%s now renders a %s", sess.view, rendering.Kind))
	}
	if sess.viewText != "" && r.DeclarationText(r.DeclaredView(sess.doc, sess.view)) != sess.viewText {
		return end(fmt.Sprintf("%s was edited", sess.view))
	}
	if doc.Version == sess.version && sameRendering(rendering, sess.rendering) {
		return false
	}
	if err := sess.locate(rendering, target, doc.Version); err != nil {
		return end(fmt.Sprintf("%s no longer draws %s: %v", sess.view, sess.target, err))
	}
	return true
}

// dependencyChange describes the first way the declarations target's run would
// read now differ from those it read: one edited, one it did not read, or one
// gone; "" for none.
func dependencyChange(r *model.Reading, target string, was, now []model.Dependency) string {
	before := make(map[dependencyKey]bool, len(was))
	for _, dep := range was {
		before[keyOf(dep)] = true
	}
	after := make(map[dependencyKey]string, len(now))
	for _, dep := range now {
		after[keyOf(dep)] = dep.Text
	}
	for _, dep := range was {
		if text, ok := after[keyOf(dep)]; ok && text != dep.Text {
			return fmt.Sprintf("%s was edited", dep.Name())
		}
	}
	for _, dep := range now {
		if !before[keyOf(dep)] {
			return fmt.Sprintf("%s now reads %s", target, dep.Name())
		}
	}
	for _, dep := range was {
		if _, ok := after[keyOf(dep)]; ok {
			continue
		}
		if r.Declared(dep.Doc, dep.FQN) == nil {
			return fmt.Sprintf("%s is no longer declared", dep.Name())
		}
		return fmt.Sprintf("%s no longer reads %s", target, dep.Name())
	}
	return ""
}

// dependencyKey names a dependency apart from its text.
type dependencyKey struct{ doc, fqn string }

func keyOf(dep model.Dependency) dependencyKey { return dependencyKey{dep.Doc, dep.FQN} }

// sameRendering reports whether two renderings draw the same nodes and edges
// under the same IDs.
func sameRendering(a, b *view.Rendering) bool {
	if a == nil || b == nil || len(a.Edges) != len(b.Edges) {
		return false
	}
	var flatten func(nodes []*view.Node, out *[]string)
	flatten = func(nodes []*view.Node, out *[]string) {
		for _, n := range nodes {
			*out = append(*out, fmt.Sprintf("%s|%s|%v", n.ID, n.Kind, n.Origin))
			flatten(n.Children, out)
		}
	}
	var an, bn []string
	flatten(a.Roots, &an)
	flatten(b.Roots, &bn)
	if len(an) != len(bn) {
		return false
	}
	for i := range an {
		if an[i] != bn[i] {
			return false
		}
	}
	for i := range a.Edges {
		if a.Edges[i].From != b.Edges[i].From || a.Edges[i].To != b.Edges[i].To || a.Edges[i].Origin != b.Edges[i].Origin {
			return false
		}
	}
	return true
}

// debugDocumentClosed ends the sessions on the closed buffer name, whether or
// not the workspace keeps the file's text from disk.
func (s *Server) debugDocumentClosed(ctx context.Context, name string) {
	s.debugSessionsEndedWhere(ctx, name+" was closed", func(sess *debugSession) bool { return sess.doc == name })
}

// debugSessionsEnded ends every live session with reason, announcing each.
func (s *Server) debugSessionsEnded(ctx context.Context, reason string) {
	s.debugSessionsEndedWhere(ctx, reason, func(*debugSession) bool { return true })
}

// debugSessionsEndedWhere ends the live sessions matching with reason, announcing each.
func (s *Server) debugSessionsEndedWhere(ctx context.Context, reason string, matching func(*debugSession) bool) {
	s.debug.mu.Lock()
	var ended []*debugSnapshot
	for id, sess := range s.debug.sessions {
		if !matching(sess) {
			continue
		}
		sess.ended = reason
		ended = append(ended, sess.snapshot())
		delete(s.debug.sessions, id)
		sess.release()
	}
	s.debug.mu.Unlock()
	s.notifyDebugChanged(ctx, ended)
}

// notifyDebugChanged sends debugChanged for each snapshot, in session order.
// The push is best-effort, like diagnostics: a client polls on its next request.
func (s *Server) notifyDebugChanged(ctx context.Context, snaps []*debugSnapshot) {
	if s.notifier == nil {
		return
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].Session < snaps[j].Session })
	for _, snap := range snaps {
		_ = s.notifier.Notify(ctx, MethodDebugChanged, snap)
	}
}
