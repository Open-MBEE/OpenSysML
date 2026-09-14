package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
// performed by object when one is named (a part or object definition or usage).
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

	rt        *runtime.Context
	runtime   *model.Runtime
	targetSym *symbols.Symbol
	objectSym *symbols.Symbol
	action    *runtime.ActionExecutor
	machine   *runtime.StateExecutor

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
	name := uriToName(params.TextDocument.URI)
	if strings.TrimSpace(params.Target) == "" {
		return nil, debugInvalid(fmt.Errorf("%w: no target named", ErrDebugTarget))
	}
	rendering, doc, err := s.ws.RenderView(name, params.View)
	if err != nil {
		return nil, err
	}
	if rendering.Kind != view.KindState && rendering.Kind != view.KindAction {
		return nil, debugInvalid(fmt.Errorf("%w: %s renders a %s, which no debugger drives", ErrDebugTarget, params.View, rendering.Kind))
	}
	rt, err := s.ws.NewRuntime()
	if err != nil {
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
	}
	if objectSym != nil {
		sess.object = rt.FQN(objectSym)
	}
	switch rendering.Kind {
	case view.KindState:
		sess.machine, err = ctx.CreateStateExecutorFor(target, performer)
	case view.KindAction:
		sess.action, err = ctx.CreateActionExecutorFor(target, performer)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrDebugTarget, params.Target, err)
	}
	if err := sess.locate(rendering, target, doc.Version); err != nil {
		sess.release()
		return nil, err
	}
	s.debug.mu.Lock()
	s.debug.next++
	sess.id = "debug-" + strconv.Itoa(s.debug.next)
	s.debug.sessions[sess.id] = sess
	snap := sess.snapshot()
	s.debug.mu.Unlock()
	return snap, nil
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
		loc, err := view.LocateActions(rendering, drawn, sess.targetSym)
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
			if old.node == bp.node {
				kept[id] = bp
			}
		}
	}
	sess.breakpoints = kept
	sess.applyBreakpoints()
	return nil
}

// applyBreakpoints sets the executor's breakpoints to the session's.
func (sess *debugSession) applyBreakpoints() {
	if sess.action == nil {
		return
	}
	sess.action.ClearBreakpoints()
	for _, bp := range sess.breakpoints {
		sess.action.SetBreakpointAt(bp.node)
	}
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
		err := sess.action.Step()
		if errors.Is(err, runtime.ErrNothingDue) {
			sess.waiting = debugClockWait(sess.action)
		} else if err != nil {
			sess.failure = err.Error()
		}
	case view.KindState:
		moved, err := sess.stepMachine()
		if err != nil {
			sess.failure = err.Error()
		} else if moved {
			sess.pauseMachine()
		}
	}
	sess.pause()
	return sess.snapshot(), nil
}

// resume forgets what the last run ended on before the next one.
func (sess *debugSession) resume() {
	sess.waiting, sess.failure = "", ""
	sess.paused, sess.pausedName = "", ""
}

// pause records the breakpoint node an action run just stopped at, if any; a
// machine records its own as it suspends (pauseMachine).
func (sess *debugSession) pause() {
	if sess.action == nil || sess.action.PausedAt() == "" {
		return
	}
	for _, tok := range sess.action.Tokens() {
		if sess.isBreakpoint(tok.Location) {
			sess.paused, _ = sess.actions.Node(tok.Within(), tok.Location)
			sess.pausedName = sess.action.PausedAt()
			return
		}
	}
}

// pauseMachine suspends the machine at the first active breakpoint state,
// reporting whether one was hit.
func (sess *debugSession) pauseMachine() bool {
	if len(sess.breakpoints) == 0 {
		return false
	}
	for _, state := range sess.machine.ActiveStates() {
		if !sess.isBreakpoint(state) {
			continue
		}
		sess.machine.Suspend()
		sess.paused, _ = sess.states.Node(state)
		sess.pausedName = state.Name
		return true
	}
	return false
}

// debugClockWait says what an action parked on the clock waits for.
func debugClockWait(exec *runtime.ActionExecutor) string {
	if due, ok := exec.NextWait(); ok {
		return "waits on the clock until t=" + semantics.FormatReal(due)
	}
	return "waits on the clock"
}

// stepMachine advances a state machine one step as the REPL does, reporting
// whether anything moved.
func (sess *debugSession) stepMachine() (bool, error) {
	exec := sess.machine
	if exec.HasPendingWork() || exec.WatchesChangeCondition() {
		exec.Resume()
	}
	if exec.State() != runtime.StateRunning {
		return false, nil
	}
	fired, err := exec.PollChangeEvents()
	if err != nil {
		return false, fmt.Errorf("change condition failed: %w", err)
	}
	if fired {
		return true, nil
	}
	if exec.EventQueue().Len() > 0 || exec.HasPendingSignal() {
		if err := exec.ProcessNextEvent(); err != nil {
			return false, fmt.Errorf("event processing failed: %w", err)
		}
		return true, nil
	}
	if exec.HasPendingDoWork() {
		ran, err := exec.RunDoRound()
		if err != nil {
			return false, fmt.Errorf("do behavior failed: %w", err)
		}
		return ran > 0, nil
	}
	exec.Suspend()
	return false, nil
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

// continueMachine steps a state machine until nothing moves at the current
// instant, a breakpoint state becomes active, or the event budget is spent.
func (sess *debugSession) continueMachine() {
	budget := runtime.DefaultBudgets().MaxStateEvents
	for i := int64(0); i < budget; i++ {
		moved, err := sess.stepMachine()
		if err != nil {
			sess.failure = err.Error()
			return
		}
		if !moved || sess.pauseMachine() {
			return
		}
	}
	sess.failure = fmt.Sprintf("state machine did not settle within %d events", budget)
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
	msg, err := sess.signalMessage(params.Signal, params.Args)
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

// signalMessage builds the message a send posts: typed by the signal definition
// named, or by name alone when none is declared and no arguments are given.
func (sess *debugSession) signalMessage(signal string, args map[string]string) (runtime.Message, error) {
	sym := sess.signalDefinition(signal)
	if sym == nil {
		if len(args) > 0 {
			return runtime.Message{}, fmt.Errorf("no signal definition %s is declared, so it cannot carry arguments", signal)
		}
		return runtime.NamedSignalMessage(signal, sess.performer()), nil
	}
	if !runtime.IsSignalDefinition(sym) {
		return runtime.Message{}, fmt.Errorf("%s is a %s, not a signal definition", signal, sym.Notation())
	}
	bound := make(map[string]runtime.Value, len(args))
	names := make([]string, 0, len(args))
	for name := range args {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		expr, err := parseDebugExpression(args[name])
		if err != nil {
			return runtime.Message{}, fmt.Errorf("argument %s: %w", name, err)
		}
		value, err := sess.rt.EvalWithScope(expr, sess.targetSym.OwnerScope)
		if err != nil {
			return runtime.Message{}, fmt.Errorf("argument %s: %w", name, err)
		}
		bound[name] = value
	}
	return sess.rt.SignalMessage(sym, bound, sess.performer())
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
	if _, err := sess.rt.Advance(params.Time); err != nil {
		sess.failure = err.Error()
	} else if sess.machine != nil {
		sess.pauseMachine()
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
	var notes []runtime.RunNote
	switch sess.kind {
	case view.KindAction:
		snap.Root = sess.actions.Root()
		sess.actionSnapshot(snap)
		notes = sess.action.Notes()
	case view.KindState:
		snap.Root = sess.states.Root()
		sess.machineSnapshot(snap)
		notes = sess.machine.Notes()
	}
	if len(notes) > sess.noted {
		for _, note := range notes[sess.noted:] {
			snap.Notes = append(snap.Notes, note.String())
		}
		sess.noted = len(notes)
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
	traversals := exec.Traversals()
	if len(traversals) > sess.fired {
		for _, tr := range traversals[sess.fired:] {
			if edge, ok := sess.actionEdge(tr.Within, tr.Edge); ok {
				snap.Taken = append(snap.Taken, edge)
			}
		}
		sess.fired = len(traversals)
	}
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

// isBreakpoint reports whether node is one of the session's breakpoints.
func (sess *debugSession) isBreakpoint(node ast.Node) bool {
	for _, bp := range sess.breakpoints {
		if bp.node == node {
			return true
		}
	}
	return false
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
	fired := exec.FiredTransitions()
	if len(fired) > sess.fired {
		for _, f := range fired[sess.fired:] {
			if index, ok := sess.states.Transition(f.Decl, f.Source, f.Target); ok {
				snap.Taken = append(snap.Taken, sess.edgeAt(index))
			}
		}
		sess.fired = len(fired)
	}
	for _, event := range exec.EventQueue().Events() {
		snap.Queue = append(snap.Queue, debugEvent{Event: debugEventText(event), At: event.Timestamp})
	}
	for _, msg := range sess.rt.PendingMessages() {
		accepted, err := exec.AcceptsMessage(msg)
		if err != nil || !accepted {
			continue
		}
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
	end := func(reason string) (*debugSnapshot, bool) {
		sess.ended = reason
		return sess.snapshot(), true
	}
	doc := s.ws.Document(sess.doc)
	if doc == nil {
		return end(fmt.Sprintf("%s was closed", sess.doc))
	}
	target := s.ws.Declared(sess.doc, sess.target)
	if target == nil {
		return end(fmt.Sprintf("%s is no longer declared", sess.target))
	}
	if s.ws.DeclarationText(target) != sess.runtime.Text(sess.targetSym) {
		return end(fmt.Sprintf("%s was edited", sess.target))
	}
	if sess.objectSym != nil {
		object := s.ws.Declared(sess.doc, sess.object)
		if object == nil {
			return end(fmt.Sprintf("%s is no longer declared", sess.object))
		}
		if s.ws.DeclarationText(object) != sess.runtime.Text(sess.objectSym) {
			return end(fmt.Sprintf("%s was edited", sess.object))
		}
	}
	rendering, doc, err := s.ws.RenderView(sess.doc, sess.view)
	if err != nil {
		return end(fmt.Sprintf("%s no longer renders: %v", sess.view, err))
	}
	if rendering.Kind != sess.kind {
		return end(fmt.Sprintf("%s now renders a %s", sess.view, rendering.Kind))
	}
	if doc.Version == sess.version && sameRendering(rendering, sess.rendering) {
		return nil, false
	}
	if err := sess.locate(rendering, target, doc.Version); err != nil {
		return end(fmt.Sprintf("%s no longer draws %s: %v", sess.view, sess.target, err))
	}
	return sess.snapshot(), true
}

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

// debugSessionsEnded ends every live session with reason, announcing each.
func (s *Server) debugSessionsEnded(ctx context.Context, reason string) {
	s.debug.mu.Lock()
	var ended []*debugSnapshot
	for id, sess := range s.debug.sessions {
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
