package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

// debugMachine is a parallel state machine: one region moves on a guarded
// signal into a composite state, the other on the clock.
const debugMachine = `package Machines {
	private import ScalarValues::*;
	private import SI::*;

	attribute def Go { attribute level : Integer; }
	attribute def Halt;

	state def Ops parallel {
		attribute threshold : Integer = 1;
		state motion {
			entry; then idle;
			state idle;
			state busy {
				entry; then working;
				state working;
				state finishing;
				transition first working accept Halt then finishing;
			}
			transition go first idle accept g : Go if g.level > threshold then busy;
		}
		state clock {
			entry; then waiting;
			state waiting;
			accept after 5 [s] then elapsed;
			state elapsed;
		}
	}

	part def Robot {
		exhibit state ops : Ops;
	}
}
package MachineViews {
	private import StandardViewDefinitions::*;
	view opsView : StateTransitionView { expose Machines::Ops; }
}
`

// debugFlow is an action forking into a nested flow, an assignment, a timed
// wait and a signal wait, joining and deciding on what the assignment did.
const debugFlow = `package Flows {
	private import ScalarValues::*;
	private import SI::*;
	attribute def Ping;
	action def Drive {
		attribute speed : Integer = 0;
		first start;
		fork split;
		action prep { first begin; action warm; succession first begin then warm; }
		action tally { assign speed := speed + 1; }
		action pause accept after 5 [s];
		action listen accept Ping;
		join sync;
		action park;
		done;
		succession first start then split;
		succession first split then prep;
		succession first split then tally;
		succession first split then pause;
		succession first split then listen;
		succession first prep then sync;
		succession first tally then sync;
		succession first pause then sync;
		succession first listen then sync;
		decide check;
		succession first sync then check;
		if speed > 0 then done;
		else park;
		succession first park then done;
	}
}
package FlowViews {
	private import StandardViewDefinitions::*;
	view driveView : ActionFlowView { expose Flows::Drive; }
}
`

// debugRecorder is a client that keeps the debugChanged snapshots it is sent.
type debugRecorder struct {
	recorder
	mu      sync.Mutex
	changed []*debugSnapshot
}

func (r *debugRecorder) Notify(ctx context.Context, method string, params interface{}) error {
	if method == MethodDebugChanged {
		if snap, ok := params.(*debugSnapshot); ok {
			r.mu.Lock()
			r.changed = append(r.changed, snap)
			r.mu.Unlock()
		}
	}
	return r.recorder.Notify(ctx, method, params)
}

func (r *debugRecorder) debugChanged() []*debugSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*debugSnapshot(nil), r.changed...)
}

// debugServer is a server holding one open document, with a client that keeps
// the debugChanged notifications.
func debugServer(t *testing.T, name, src string) (*Server, uri.URI, *debugRecorder) {
	t.Helper()
	s := NewServer(model.NewWorkspace())
	rec := &debugRecorder{}
	s.client = rec
	s.notifier = rec
	docURI := uri.File(name)
	if err := s.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, LanguageID: "sysml", Version: 1, Text: src},
	}); err != nil {
		t.Fatalf("DidOpen err = %v", err)
	}
	return s, docURI, rec
}

// debugCall dispatches a debug request through the handler chain, as a served
// session does, and decodes the snapshot the client would receive.
func debugCall(t *testing.T, s *Server, method string, params any) (*debugSnapshot, error) {
	t.Helper()
	req, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(1), method, params)
	if err != nil {
		t.Fatalf("build %s request: %v", method, err)
	}
	var (
		raw     json.RawMessage
		callErr error
	)
	reply := func(ctx context.Context, result interface{}, err error) error {
		if err != nil {
			callErr = err
			return nil
		}
		encoded, mErr := json.Marshal(result)
		if mErr != nil {
			t.Fatalf("marshal %s result: %v", method, mErr)
		}
		raw = encoded
		return nil
	}
	handler := s.debugHandler(func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		t.Fatalf("%s was not handled: it fell through to the next handler", req.Method())
		return nil
	})
	if err := handler(context.Background(), reply, req); err != nil {
		t.Fatalf("dispatch %s: %v", method, err)
	}
	if callErr != nil {
		return nil, callErr
	}
	var snap debugSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("decode %s result %s: %v", method, raw, err)
	}
	return &snap, nil
}

// mustDebug is debugCall for a request that must succeed.
func mustDebug(t *testing.T, s *Server, method string, params any) *debugSnapshot {
	t.Helper()
	snap, err := debugCall(t, s, method, params)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return snap
}

// ids maps the names of a rendering's nodes to their IDs; names must be unique.
func ids(t *testing.T, r *renderResult) map[string]string {
	t.Helper()
	out := make(map[string]string, len(r.Nodes))
	for _, n := range r.Nodes {
		if n.Name == "" {
			continue
		}
		if _, dup := out[n.Name]; dup {
			t.Fatalf("node name %q is drawn twice; the fixture must name nodes uniquely", n.Name)
		}
		out[n.Name] = n.ID
	}
	return out
}

// edgeIndex is the position of the edge from one node to another in a render
// result, which is what a snapshot's edges are indexed by.
func edgeIndex(t *testing.T, r *renderResult, from, to string) int {
	t.Helper()
	for i, e := range r.Edges {
		if e.From == from && e.To == to {
			return i
		}
	}
	t.Fatalf("no edge %s -> %s among %d edges", from, to, len(r.Edges))
	return -1
}

// edges spells a snapshot's edges as from->to for comparison.
func edges(list []debugEdge) []string {
	out := make([]string, 0, len(list))
	for _, e := range list {
		out = append(out, e.From+"->"+e.To)
	}
	return out
}

// tokenAt is the token standing at node, failing when none does.
func debugTokenAt(t *testing.T, snap *debugSnapshot, node string) debugToken {
	t.Helper()
	for _, tok := range snap.Tokens {
		if tok.Node == node {
			return tok
		}
	}
	t.Fatalf("no token at %s in %s", node, describe(snap))
	return debugToken{}
}

// describe writes a snapshot for a failure message.
func describe(snap *debugSnapshot) string {
	b, _ := json.Marshal(snap)
	return string(b)
}

func wantStrings(t *testing.T, what string, got, want []string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func wantState(t *testing.T, snap *debugSnapshot, state string) {
	t.Helper()
	if snap.State != state {
		t.Errorf("state = %q (%s), want %q: %s", snap.State, snap.Reason, state, describe(snap))
	}
}

// The request and snapshot shapes cross the wire under their documented names.
func TestDebugProtocolShapes(t *testing.T) {
	start := debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File("/w/m.sysml")},
		View:         "V::v", Target: "P::T", Object: "P::O",
	}
	raw, err := json.Marshal(start)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"textDocument", "view", "target", "object"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("start params lack %q: %s", key, raw)
		}
	}
	var back debugStartParams
	if err := json.Unmarshal(raw, &back); err != nil || back != start {
		t.Errorf("start params round trip = %+v, %v; want %+v", back, err, start)
	}

	bp := debugBreakpointsParams{Session: "debug-1", NodeIDs: []string{"n3", "n5"}}
	raw, _ = json.Marshal(bp)
	if want := `{"session":"debug-1","nodeIds":["n3","n5"]}`; string(raw) != want {
		t.Errorf("breakpoints params = %s, want %s", raw, want)
	}
	send := debugSendParams{Session: "debug-1", Signal: "Go", Args: map[string]string{"level": "3"}}
	raw, _ = json.Marshal(send)
	if want := `{"session":"debug-1","signal":"Go","args":{"level":"3"}}`; string(raw) != want {
		t.Errorf("send params = %s, want %s", raw, want)
	}
	advance := debugAdvanceParams{Session: "debug-1", Time: 2.5}
	raw, _ = json.Marshal(advance)
	if want := `{"session":"debug-1","time":2.5}`; string(raw) != want {
		t.Errorf("advance params = %s, want %s", raw, want)
	}

	due := 5.0
	snap := debugSnapshot{
		Protocol: debugProtocolVersion, Session: "debug-1", Kind: "action", View: "V::v", Target: "P::T",
		Root: "n0", Version: 3, State: debugWaiting, Reason: "Ping", Time: 5,
		Tokens: []debugToken{{ID: 2, Node: "n4", Placed: true, Via: &debugEdge{Index: 1, From: "n1", To: "n4"},
			Awaiting: []debugEdge{{Index: 2, From: "n2", To: "n4"}}, Waiting: "until t=5.0", Due: &due}},
		ActiveStates: []string{}, Taken: []debugEdge{{Index: 1, From: "n1", To: "n4"}},
		Queue:       []debugEvent{{Event: "Ping", At: 5, Pending: true}},
		Breakpoints: []string{"n4"}, PausedAt: "n4", Notes: []string{"a note"}, Results: map[string]string{"speed": "1"},
	}
	raw, err = json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var decoded debugSnapshot
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if again, _ := json.Marshal(decoded); string(again) != string(raw) {
		t.Errorf("snapshot round trip changed it:\n%s\n%s", raw, again)
	}
	for _, key := range []string{"protocol", "session", "kind", "view", "target", "root", "version", "state", "reason",
		"time", "tokens", "activeStates", "taken", "queue", "breakpoints", "pausedAt", "notes", "results"} {
		if !strings.Contains(string(raw), `"`+key+`":`) {
			t.Errorf("snapshot lacks %q: %s", key, raw)
		}
	}
	if strings.Contains(string(raw), `"object"`) {
		t.Errorf("an unset object is written: %s", raw)
	}
	// The empty collections a client iterates are written as such, never null.
	empty, _ := json.Marshal(debugSnapshot{Tokens: []debugToken{}, ActiveStates: []string{}, Taken: []debugEdge{},
		Queue: []debugEvent{}, Breakpoints: []string{}, Notes: []string{}})
	if strings.Contains(string(empty), "null") {
		t.Errorf("empty snapshot writes null: %s", empty)
	}
}

// A request naming a behavior the view does not draw, a session not live, a node
// not drawn or a signal not taken is refused with a typed InvalidParams error.
func TestDebugRefusesWhatItCannotRun(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/m.sysml", debugMachine+`
package Other {
	part def Widget;
	view widgets : TreeView { expose Other::Widget; }
	private import StandardViewDefinitions::*;
}
`)
	doc := protocol.TextDocumentIdentifier{URI: docURI}
	cases := []struct {
		name   string
		params debugStartParams
		want   error
	}{
		{"no target", debugStartParams{TextDocument: doc, View: "MachineViews::opsView"}, ErrDebugTarget},
		{"not a behavior view", debugStartParams{TextDocument: doc, View: "Other::widgets", Target: "Other::Widget"}, ErrDebugTarget},
		{"undeclared target", debugStartParams{TextDocument: doc, View: "MachineViews::opsView", Target: "Machines::Nope"}, ErrDebugTarget},
		{"wrong kind of target", debugStartParams{TextDocument: doc, View: "MachineViews::opsView", Target: "Machines::Robot"}, ErrDebugTarget},
		{"undeclared performer", debugStartParams{TextDocument: doc, View: "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Nope"}, ErrDebugTarget},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.DebugStart(&tc.params)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			var wire *jsonrpc2.Error
			if !errors.As(err, &wire) || wire.Code != jsonrpc2.InvalidParams {
				t.Errorf("err = %v, want an InvalidParams reply", err)
			}
		})
	}

	if _, err := s.DebugStep(&debugSessionParams{Session: "debug-99"}); !errors.Is(err, ErrDebugSession) {
		t.Errorf("step of an unknown session: err = %v, want %v", err, ErrDebugSession)
	}

	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{TextDocument: doc, View: "MachineViews::opsView", Target: "Machines::Ops"})
	if _, err := s.DebugBreakpoints(&debugBreakpointsParams{Session: snap.Session, NodeIDs: []string{"n99"}}); !errors.Is(err, ErrDebugNode) {
		t.Errorf("breakpoint on an undrawn node: err = %v, want %v", err, ErrDebugNode)
	}
	if _, err := s.DebugSend(&debugSendParams{Session: snap.Session}); !errors.Is(err, ErrDebugSignal) {
		t.Errorf("send of no signal: err = %v, want %v", err, ErrDebugSignal)
	}
	if _, err := s.DebugSend(&debugSendParams{Session: snap.Session, Signal: "Machines::Robot"}); !errors.Is(err, ErrDebugSignal) {
		t.Errorf("send of a part def: err = %v, want %v", err, ErrDebugSignal)
	}
	if _, err := s.DebugSend(&debugSendParams{Session: snap.Session, Signal: "Halt"}); !errors.Is(err, ErrDebugSignal) {
		t.Errorf("send of a signal no active state accepts: err = %v, want %v", err, ErrDebugSignal)
	}
	if _, err := s.DebugAdvance(&debugAdvanceParams{Session: snap.Session, Time: -1}); err == nil {
		t.Error("advance by a negative duration was accepted")
	}
	stopped := mustDebug(t, s, MethodDebugStop, &debugSessionParams{Session: snap.Session})
	wantState(t, stopped, debugEnded)
	if _, err := s.DebugContinue(&debugSessionParams{Session: snap.Session}); !errors.Is(err, ErrDebugSession) {
		t.Errorf("continue after stop: err = %v, want %v", err, ErrDebugSession)
	}
}

// A state machine session reports active states per region, fired transitions
// and queued events in render IDs; signals, the clock and breakpoints drive it.
func TestDebugStateMachineSession(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/m.sysml", debugMachine)
	r := render(t, s, docURI, "MachineViews::opsView")
	id := ids(t, r)
	edge := func(from, to string) string { return id[from] + "->" + id[to] }
	startOf := func(parent string) string {
		for _, n := range r.Nodes {
			if n.Kind == "start" && n.Parent == id[parent] {
				return n.ID
			}
		}
		t.Fatalf("no start marker in %s", parent)
		return ""
	}

	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Robot",
	})
	if snap.Protocol != debugProtocolVersion || snap.Kind != "state" || snap.Root != r.Nodes[0].ID || snap.Version != r.Version {
		t.Errorf("start snapshot header = %s", describe(snap))
	}
	if snap.Object != "Machines::Robot" || snap.Target != "Machines::Ops" {
		t.Errorf("start snapshot names %q performed by %q", snap.Target, snap.Object)
	}
	wantState(t, snap, debugRunning)
	wantStrings(t, "initial active states", snap.ActiveStates, []string{id["motion"], id["idle"], id["clock"], id["waiting"]})
	wantStrings(t, "entry transitions", edges(snap.Taken), []string{startOf("motion") + "->" + id["idle"], startOf("clock") + "->" + id["waiting"]})
	if len(snap.Queue) != 1 || snap.Queue[0].At != 5 || snap.Queue[0].Pending {
		t.Errorf("queue = %+v, want the time event due at 5", snap.Queue)
	}

	// One step dispatches the queued time event, moving the clock to it.
	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugRunning)
	if snap.Time != 5 {
		t.Errorf("time = %v, want 5", snap.Time)
	}
	wantStrings(t, "active after the time event", snap.ActiveStates, []string{id["motion"], id["idle"], id["clock"], id["elapsed"]})
	wantStrings(t, "taken by the time event", edges(snap.Taken), []string{edge("waiting", "elapsed")})
	if len(snap.Queue) != 0 {
		t.Errorf("queue = %+v, want it drained", snap.Queue)
	}

	// With nothing left to do a step quiesces the machine, and says so.
	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugSuspended)
	if !strings.Contains(snap.Reason, "quiesced") {
		t.Errorf("reason = %q, want quiescence", snap.Reason)
	}
	if len(snap.Taken) != 0 {
		t.Errorf("a quiescent step reports taken = %v", edges(snap.Taken))
	}

	// A signal is posted, shown pending, and delivered by the next run; a
	// guard that fails leaves the machine where it was.
	snap = mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: snap.Session, Signal: "Go", Args: map[string]string{"level": "0"}})
	if len(snap.Queue) != 1 || snap.Queue[0].Event != "Go(level=0)" || !snap.Queue[0].Pending {
		t.Errorf("queue after send = %+v, want the pending Go", snap.Queue)
	}
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugSuspended)
	wantStrings(t, "active after a refused guard", snap.ActiveStates, []string{id["motion"], id["idle"], id["clock"], id["elapsed"]})

	// A guard reading a signal feature the message never carried fails the
	// run, which the snapshot reports rather than hiding.
	snap = mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: snap.Session, Signal: "Go"})
	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugFailed)
	if snap.Reason == "" {
		t.Error("a failed run gives no reason")
	}

	// A passing guard takes the transition into the composite state and its entry
	// transition, in order; the composite state is active with its substate.
	snap = mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: snap.Session, Signal: "Go", Args: map[string]string{"level": "1 + 2"}})
	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugRunning)
	wantStrings(t, "active in the composite state", snap.ActiveStates, []string{id["motion"], id["busy"], id["working"], id["clock"], id["elapsed"]})
	wantStrings(t, "taken into the composite state", edges(snap.Taken), []string{edge("idle", "busy"), startOf("busy") + "->" + id["working"]})

	// The clock advances without anything due; the machine stays put.
	snap = mustDebug(t, s, MethodDebugAdvance, &debugAdvanceParams{Session: snap.Session, Time: 5})
	if snap.Time != 10 {
		t.Errorf("time after advance = %v, want 10", snap.Time)
	}
	wantStrings(t, "active after advance", snap.ActiveStates, []string{id["motion"], id["busy"], id["working"], id["clock"], id["elapsed"]})

	// A breakpoint on a state stops a continue as the state becomes active.
	snap = mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: snap.Session, NodeIDs: []string{id["finishing"]}})
	wantStrings(t, "breakpoints", snap.Breakpoints, []string{id["finishing"]})
	mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: snap.Session, Signal: "Halt"})
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != id["finishing"] || !strings.Contains(snap.Reason, "breakpoint finishing") {
		t.Errorf("pausedAt = %q reason = %q, want the breakpoint on finishing", snap.PausedAt, snap.Reason)
	}
	wantStrings(t, "active at the breakpoint", snap.ActiveStates, []string{id["motion"], id["busy"], id["finishing"], id["clock"], id["elapsed"]})
	wantStrings(t, "taken to the breakpoint", edges(snap.Taken), []string{edge("working", "finishing")})

	// The pause is reported once: the next run quiesces without a breakpoint.
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != "" || strings.Contains(snap.Reason, "breakpoint") {
		t.Errorf("a quiescent run after the pause still reports pausedAt = %q reason = %q", snap.PausedAt, snap.Reason)
	}

	snap = mustDebug(t, s, MethodDebugStop, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugEnded)
	if snap.Reason != "stopped" {
		t.Errorf("stop reason = %q", snap.Reason)
	}
}

// An action session reports every token (forked, nested, parked on the clock or
// a signal, held at a join) with the edges it took, in the rendering's IDs.
func TestDebugActionSession(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/f.sysml", debugFlow)
	r := render(t, s, docURI, "FlowViews::driveView")
	id := ids(t, r)
	edge := func(from, to string) string { return id[from] + "->" + id[to] }

	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "FlowViews::driveView", Target: "Flows::Drive",
	})
	if snap.Kind != "action" || snap.Root != r.Nodes[0].ID || snap.Object != "" {
		t.Errorf("start snapshot header = %s", describe(snap))
	}
	wantState(t, snap, debugRunning)
	if len(snap.Tokens) != 1 || snap.Tokens[0].Node != id["start"] || !snap.Tokens[0].Placed || snap.Tokens[0].Via != nil {
		t.Errorf("initial tokens = %+v, want one at start", snap.Tokens)
	}

	// One step moves the one token one succession on.
	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugRunning)
	tok := debugTokenAt(t, snap, id["split"])
	if tok.Via == nil || tok.Via.Index != edgeIndex(t, r, id["start"], id["split"]) {
		t.Errorf("token at split arrived via %+v, want the edge from start", tok.Via)
	}
	wantStrings(t, "taken by one step", edges(snap.Taken), []string{edge("start", "split")})

	// A breakpoint inside the nested flow stops the run as a token reaches
	// it, while the other branches have parked or joined.
	snap = mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: snap.Session, NodeIDs: []string{id["begin"]}})
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != id["begin"] || !strings.Contains(snap.Reason, "breakpoint begin") {
		t.Errorf("pausedAt = %q reason = %q, want the breakpoint on begin", snap.PausedAt, snap.Reason)
	}
	if len(snap.Tokens) != 4 {
		t.Errorf("tokens = %d, want the fork's four branches: %s", len(snap.Tokens), describe(snap))
	}
	nested := debugTokenAt(t, snap, id["begin"])
	if !nested.Placed {
		t.Errorf("the token in the nested flow is not placed: %+v", nested)
	}
	joined := debugTokenAt(t, snap, id["sync"])
	wantStrings(t, "join edges awaited", edges(joined.Awaiting), []string{edge("prep", "sync"), edge("pause", "sync"), edge("listen", "sync")})
	timed := debugTokenAt(t, snap, id["pause"])
	if timed.Due == nil || *timed.Due != 5 || !strings.Contains(timed.Waiting, "t=5") {
		t.Errorf("timed wait = %+v, want due at 5", timed)
	}
	if listening := debugTokenAt(t, snap, id["listen"]); listening.Waiting != "Ping" || listening.Due != nil {
		t.Errorf("signal wait = %+v, want Ping", listening)
	}
	taken := edges(snap.Taken)
	for _, want := range []string{edge("split", "prep"), edge("split", "tally"), edge("split", "pause"), edge("split", "listen"), edge("tally", "sync")} {
		if !contains(taken, want) {
			t.Errorf("taken %v lacks %s", taken, want)
		}
	}

	// Cleared, the breakpoint no longer holds the run, which now waits on
	// the clock for the timed accept.
	snap = mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: snap.Session})
	if len(snap.Breakpoints) != 0 {
		t.Errorf("breakpoints after clearing = %v", snap.Breakpoints)
	}
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugWaiting)
	if !strings.Contains(snap.Reason, "t=5") {
		t.Errorf("reason = %q, want the clock wait", snap.Reason)
	}
	taken = edges(snap.Taken)
	for _, want := range []string{edge("begin", "warm"), edge("prep", "sync")} {
		if !contains(taken, want) {
			t.Errorf("taken %v lacks %s", taken, want)
		}
	}

	// Advancing to the due instant releases the timed branch; the signal
	// branch still waits.
	snap = mustDebug(t, s, MethodDebugAdvance, &debugAdvanceParams{Session: snap.Session, Time: 5})
	wantState(t, snap, debugWaiting)
	if snap.Time != 5 || snap.Reason != "Ping" {
		t.Errorf("after advance time = %v reason = %q, want 5 and Ping", snap.Time, snap.Reason)
	}
	wantStrings(t, "taken by the clock", edges(snap.Taken), []string{edge("pause", "sync")})
	if len(debugTokenAt(t, snap, id["sync"]).Awaiting) != 1 {
		t.Errorf("join awaits %+v, want only the signal branch", debugTokenAt(t, snap, id["sync"]).Awaiting)
	}

	// The signal arrives, the join fires, the decision reads the assignment.
	snap = mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: snap.Session, Signal: "Ping"})
	if len(snap.Queue) != 1 || snap.Queue[0].Event != "Ping" || !snap.Queue[0].Pending {
		t.Errorf("queue after send = %+v", snap.Queue)
	}
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugCompleted)
	if len(snap.Tokens) != 0 || snap.Results["speed"] != "1" {
		t.Errorf("completed snapshot = %s", describe(snap))
	}
	wantStrings(t, "taken to completion", edges(snap.Taken), []string{edge("listen", "sync"), edge("sync", "check"), edge("check", "done")})

	// Stepping a completed action changes nothing.
	again := mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: snap.Session})
	wantState(t, again, debugCompleted)
	if len(again.Taken) != 0 {
		t.Errorf("a step after completion took %v", edges(again.Taken))
	}
}

// An action that fails at a step reports the failure; a run that exceeds its
// budget does too, as a failure rather than a hang.
func TestDebugActionFailure(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/f.sysml", `package Flows {
	private import ScalarValues::*;
	action def Spin {
		attribute n : Integer = 0;
		first start;
		action bump { assign n := n + 1; }
		action fail { assign n := n / 0; }
		done;
		succession first start then bump;
		succession first bump then fail;
		succession first fail then done;
	}
	action def Loop {
		first start;
		action again;
		action more;
		succession first start then again;
		succession first again then more;
		succession first more then again;
	}
}
package FlowViews {
	private import StandardViewDefinitions::*;
	view spin : ActionFlowView { expose Flows::Spin; }
	view loop : ActionFlowView { expose Flows::Loop; }
}
`)
	doc := protocol.TextDocumentIdentifier{URI: docURI}
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{TextDocument: doc, View: "FlowViews::spin", Target: "Flows::Spin"})
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugFailed)
	if !strings.Contains(snap.Reason, "zero") {
		t.Errorf("reason = %q, want the division by zero", snap.Reason)
	}
	// The failure stands until the next run; the token stays where it failed.
	snap = mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: snap.Session})
	wantState(t, snap, debugFailed)
	if len(snap.Tokens) != 1 || snap.Tokens[0].Node != ids(t, render(t, s, docURI, "FlowViews::spin"))["fail"] {
		t.Errorf("tokens after the failure = %+v, want one at fail", snap.Tokens)
	}

	snap = mustDebug(t, s, MethodDebugStart, &debugStartParams{TextDocument: doc, View: "FlowViews::loop", Target: "Flows::Loop"})
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: snap.Session})
	wantState(t, snap, debugFailed)
	if !strings.Contains(snap.Reason, "steps") {
		t.Errorf("reason = %q, want the step budget", snap.Reason)
	}
}

// An edit leaving the behavior, performer and view as they were moves the session
// to the fresh IDs, announced by debugChanged; one to the behavior ends it.
func TestDebugSessionFollowsEdits(t *testing.T) {
	s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
	ctx := context.Background()
	name := uriToName(docURI)
	r := render(t, s, docURI, "MachineViews::opsView")
	id := ids(t, r)

	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Robot",
	})
	session := snap.Session
	mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{id["finishing"]}})
	before := mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session})
	wantStrings(t, "active before the edit", before.ActiveStates, []string{id["motion"], id["idle"], id["clock"], id["elapsed"]})

	// Text added above the machine shifts every declaration after it.
	shifted := "package Extra {\n\tpart def Spare;\n\tpart def Wheel;\n}\n" + debugMachine
	s.applyDidChange(ctx, name, []rawContentChange{{Text: shifted}}, 2)
	changed := rec.debugChanged()
	if len(changed) != 1 || changed[0].Session != session {
		t.Fatalf("debugChanged after an unrelated edit = %v, want one for %s", changed, session)
	}
	moved := changed[0]
	if moved.State == debugEnded {
		t.Fatalf("an unrelated edit ended the session: %s", moved.Reason)
	}
	if moved.Version != 2 {
		t.Errorf("moved snapshot is at version %d, want 2", moved.Version)
	}
	fresh := ids(t, render(t, s, docURI, "MachineViews::opsView"))
	wantStrings(t, "active after the edit", moved.ActiveStates, []string{fresh["motion"], fresh["idle"], fresh["clock"], fresh["elapsed"]})
	wantStrings(t, "breakpoints after the edit", moved.Breakpoints, []string{fresh["finishing"]})
	if len(moved.Taken) != 0 {
		t.Errorf("a relocation reports taken = %v", edges(moved.Taken))
	}

	// The session still runs, against the fresh IDs.
	mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: session, Signal: "Go", Args: map[string]string{"level": "2"}})
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
	wantStrings(t, "active after continuing", snap.ActiveStates, []string{fresh["motion"], fresh["busy"], fresh["working"], fresh["clock"], fresh["elapsed"]})

	// An edit inside another declaration of the same document is unrelated too.
	s.applyDidChange(ctx, name, []rawContentChange{{Text: strings.Replace(shifted, "part def Wheel;", "part def Wheel { attribute r : Real; }", 1)}}, 3)
	if changed = rec.debugChanged(); len(changed) != 2 || changed[1].State == debugEnded {
		t.Fatalf("debugChanged after a second unrelated edit = %v", changed)
	}

	// Rewriting the machine ends the session: its execution no longer stands
	// for what the document declares.
	edited := strings.Replace(shifted, "state finishing;", "state finishing;\n\t\t\t\tstate cooling;", 1)
	s.applyDidChange(ctx, name, []rawContentChange{{Text: edited}}, 4)
	changed = rec.debugChanged()
	if len(changed) != 3 {
		t.Fatalf("debugChanged after editing the machine = %d notifications, want 3", len(changed))
	}
	ended := changed[2]
	wantState(t, ended, debugEnded)
	if !strings.Contains(ended.Reason, "Machines::Ops was edited") {
		t.Errorf("end reason = %q", ended.Reason)
	}
	if _, err := s.DebugStep(&debugSessionParams{Session: session}); !errors.Is(err, ErrDebugSession) {
		t.Errorf("step after the end: err = %v, want %v", err, ErrDebugSession)
	}
}

// Editing the performer, removing the view or closing the document ends a
// session, each saying why; shutting the server down ends them all.
func TestDebugSessionEnds(t *testing.T) {
	ctx := context.Background()
	start := func(t *testing.T, s *Server, docURI uri.URI) string {
		t.Helper()
		snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Robot",
		})
		return snap.Session
	}
	lastEnd := func(t *testing.T, rec *debugRecorder, session string) *debugSnapshot {
		t.Helper()
		changed := rec.debugChanged()
		if len(changed) == 0 {
			t.Fatal("no debugChanged was sent")
		}
		last := changed[len(changed)-1]
		if last.Session != session || last.State != debugEnded {
			t.Fatalf("last debugChanged = %s, want %s ended", describe(last), session)
		}
		return last
	}

	t.Run("performer edited", func(t *testing.T) {
		s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
		session := start(t, s, docURI)
		s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: strings.Replace(debugMachine, "exhibit state ops : Ops;", "exhibit state ops : Ops;\n\t\tattribute id : Integer;", 1)}}, 2)
		if reason := lastEnd(t, rec, session).Reason; !strings.Contains(reason, "Machines::Robot was edited") {
			t.Errorf("reason = %q", reason)
		}
	})
	t.Run("view removed", func(t *testing.T) {
		s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
		session := start(t, s, docURI)
		s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: strings.Replace(debugMachine, "view opsView : StateTransitionView { expose Machines::Ops; }", "", 1)}}, 2)
		if reason := lastEnd(t, rec, session).Reason; !strings.Contains(reason, "MachineViews::opsView") {
			t.Errorf("reason = %q", reason)
		}
	})
	t.Run("document closed", func(t *testing.T) {
		s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
		session := start(t, s, docURI)
		if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: docURI}}); err != nil {
			t.Fatal(err)
		}
		if reason := lastEnd(t, rec, session).Reason; !strings.Contains(reason, "closed") {
			t.Errorf("reason = %q", reason)
		}
	})
	t.Run("server shut down", func(t *testing.T) {
		s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
		first, second := start(t, s, docURI), start(t, s, docURI)
		if err := s.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		changed := rec.debugChanged()
		if len(changed) != 2 || changed[0].Session != first || changed[1].Session != second {
			t.Fatalf("debugChanged on shutdown = %v, want %s then %s", changed, first, second)
		}
		for _, snap := range changed {
			wantState(t, snap, debugEnded)
		}
		if _, err := s.DebugStep(&debugSessionParams{Session: first}); !errors.Is(err, ErrDebugSession) {
			t.Errorf("step after shutdown: err = %v", err)
		}
	})
}

// Sessions run in runtimes of their own: two over one machine do not share
// state, and rendering the view they draw on is unchanged by their running.
func TestDebugSessionsAreIsolated(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/m.sysml", debugMachine)
	plain := render(t, s, docURI, "MachineViews::opsView")
	params := &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Robot",
	}
	first := mustDebug(t, s, MethodDebugStart, params)
	second := mustDebug(t, s, MethodDebugStart, params)
	if first.Session == second.Session {
		t.Fatalf("two starts share the session %s", first.Session)
	}
	stepped := mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: first.Session})
	if stepped.Time != 5 {
		t.Fatalf("first session time = %v, want 5", stepped.Time)
	}
	idle := mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: second.Session})
	if idle.Time != 0 || fmt.Sprint(idle.ActiveStates) != fmt.Sprint(second.ActiveStates) {
		t.Errorf("the second session moved with the first: %s", describe(idle))
	}
	after := render(t, s, docURI, "MachineViews::opsView")
	plainJSON, _ := json.Marshal(plain)
	afterJSON, _ := json.Marshal(after)
	if string(plainJSON) != string(afterJSON) {
		t.Errorf("rendering changed while sessions ran:\n%s\n%s", plainJSON, afterJSON)
	}
}
