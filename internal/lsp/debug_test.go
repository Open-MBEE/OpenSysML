package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
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

// advanceBy is an advance request moving session's clock forward by time.
func advanceBy(session string, time float64) *debugAdvanceParams {
	return &debugAdvanceParams{Session: session, Time: &time}
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
	raw, _ = json.Marshal(advanceBy("debug-1", 2.5))
	if want := `{"session":"debug-1","time":2.5}`; string(raw) != want {
		t.Errorf("advance params = %s, want %s", raw, want)
	}
	for _, malformed := range []string{`{"session":"debug-1"}`, `{"session":"debug-1","time":null}`} {
		var advance debugAdvanceParams
		if err := json.Unmarshal([]byte(malformed), &advance); err != nil {
			t.Fatal(err)
		}
		if advance.Time != nil {
			t.Errorf("advance params %s decoded a time of %v, want none", malformed, *advance.Time)
		}
	}

	due := 5.0
	snap := debugSnapshot{
		Protocol: debugProtocolVersion, Session: "debug-1", Kind: "action", View: "V::v", Target: "P::T",
		Root: "n0", Version: 3, Revision: 7, State: debugWaiting, Reason: "Ping", Time: 5,
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
	for _, key := range []string{"protocol", "session", "kind", "view", "target", "root", "version", "revision", "state", "reason",
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

// A request naming a behavior the view does not draw, a performer that is no
// object, a session not live, a node not drawn or a signal not taken is refused
// with a typed InvalidParams error.
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
		{"attribute as performer", debugStartParams{TextDocument: doc, View: "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Ops::threshold"}, ErrDebugTarget},
		{"signal as performer", debugStartParams{TextDocument: doc, View: "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Go"}, ErrDebugTarget},
		{"package as performer", debugStartParams{TextDocument: doc, View: "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines"}, ErrDebugTarget},
		{"behavior as performer", debugStartParams{TextDocument: doc, View: "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Ops"}, ErrDebugTarget},
		{"view as performer", debugStartParams{TextDocument: doc, View: "MachineViews::opsView", Target: "Machines::Ops", Object: "MachineViews::opsView"}, ErrDebugTarget},
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
	if _, err := s.DebugAdvance(advanceBy(snap.Session, -1)); !errors.Is(err, ErrDebugTime) {
		t.Errorf("advance by a negative duration: err = %v, want %v", err, ErrDebugTime)
	}
	if _, err := s.DebugAdvance(&debugAdvanceParams{Session: snap.Session}); !errors.Is(err, ErrDebugTime) {
		t.Errorf("advance by no duration: err = %v, want %v", err, ErrDebugTime)
	}
	if _, err := debugCall(t, s, MethodDebugAdvance, json.RawMessage(`{"session":"`+snap.Session+`","time":null}`)); !errors.Is(err, ErrDebugTime) {
		t.Errorf("advance by a null duration: err = %v, want %v", err, ErrDebugTime)
	}
	before := mustDebug(t, s, MethodDebugAdvance, advanceBy(snap.Session, 0))
	if before.Time != snap.Time {
		t.Errorf("advance by zero moved the clock from %v to %v", snap.Time, before.Time)
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
	// The robot's machine ran to quiescence at t=0 as the robot was instantiated,
	// so the session begins waiting on its timer.
	wantState(t, snap, debugWaiting)
	if !strings.Contains(snap.Reason, "t=5") {
		t.Errorf("reason = %q, want the wait until t=5", snap.Reason)
	}
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
	snap = mustDebug(t, s, MethodDebugAdvance, advanceBy(snap.Session, 5))
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

// debugDeep is a machine whose own entry transition starts in a state nested
// in a composite state, past that state's own start.
const debugDeep = `package Machines {
	attribute def Halt;
	state def Deep {
		entry; then working::step1;
		state working {
			state step1;
			state step2;
			transition first step1 accept Halt then step2;
		}
		state done;
		transition first working accept Halt then done;
	}
}
package MachineViews {
	private import StandardViewDefinitions::*;
	view deepView : StateTransitionView { expose Machines::Deep; }
}
`

// An entry transition into a nested state is taken from the start marker of the
// body it is written in — the machine's — not that of the state it lands in.
func TestDebugEntryTransitionIntoANestedState(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/m.sysml", debugDeep)
	r := render(t, s, docURI, "MachineViews::deepView")
	id := ids(t, r)
	var machineStart string
	for _, n := range r.Nodes {
		if n.Kind == "start" && n.Parent == r.Nodes[0].ID {
			machineStart = n.ID
		}
	}
	if machineStart == "" {
		t.Fatal("no start marker in the machine's own body")
	}

	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "MachineViews::deepView", Target: "Machines::Deep",
	})
	wantStrings(t, "initial active states", snap.ActiveStates, []string{id["working"], id["step1"]})
	wantStrings(t, "entry transition", edges(snap.Taken), []string{machineStart + "->" + id["step1"]})
}

// A continue runs what is due now and holds the clock: an event due later
// leaves the machine waiting, at the same instant, until an advance reaches it.
func TestDebugContinueHoldsTheClock(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/m.sysml", debugMachine)
	id := ids(t, render(t, s, docURI, "MachineViews::opsView"))
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops",
	})
	wantState(t, snap, debugRunning)
	session := snap.Session

	for i := 0; i < 2; i++ {
		snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
		wantState(t, snap, debugWaiting)
		if !strings.Contains(snap.Reason, "t=5") {
			t.Errorf("continue %d: reason = %q, want the wait until t=5", i, snap.Reason)
		}
		if snap.Time != 0 {
			t.Errorf("continue %d moved the clock to %v", i, snap.Time)
		}
		wantStrings(t, "active after a continue", snap.ActiveStates, []string{id["motion"], id["idle"], id["clock"], id["waiting"]})
		if len(snap.Queue) != 1 || snap.Queue[0].At != 5 {
			t.Errorf("continue %d: queue = %+v, want the time event still due at 5", i, snap.Queue)
		}
	}

	// A signal is delivered by a continue, the clock still held.
	mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: session, Signal: "Go", Args: map[string]string{"level": "2"}})
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
	wantState(t, snap, debugWaiting)
	if snap.Time != 0 {
		t.Errorf("delivering a signal moved the clock to %v", snap.Time)
	}
	wantStrings(t, "active after the signal", snap.ActiveStates, []string{id["motion"], id["busy"], id["working"], id["clock"], id["waiting"]})

	// Advancing to the event fires it; a continue then quiesces the machine.
	snap = mustDebug(t, s, MethodDebugAdvance, advanceBy(session, 5))
	if snap.Time != 5 {
		t.Errorf("time after advance = %v, want 5", snap.Time)
	}
	wantStrings(t, "active after the advance", snap.ActiveStates, []string{id["motion"], id["busy"], id["working"], id["clock"], id["elapsed"]})
	wantStrings(t, "taken by the advance", edges(snap.Taken), []string{id["waiting"] + "->" + id["elapsed"]})
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
	wantState(t, snap, debugSuspended)
	if !strings.Contains(snap.Reason, "quiesced") {
		t.Errorf("reason = %q, want quiescence", snap.Reason)
	}
}

// debugTransient is a machine whose timed transition enters a state it leaves
// again at the same instant.
const debugTransient = `package Transient {
	private import SI::*;
	state def Descent {
		entry; then coasting;
		state coasting;
		accept after 5 [s] then braking;
		state braking;
		then landed;
		state landed;
	}
}
package TransientViews {
	private import StandardViewDefinitions::*;
	view descentView : StateTransitionView { expose Transient::Descent; }
}
`

// An advance pauses at a breakpoint state as it becomes active, a state left
// again at the same instant included, the clock held at that instant.
func TestDebugAdvancePausesOnATransientState(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/t.sysml", debugTransient)
	id := ids(t, render(t, s, docURI, "TransientViews::descentView"))
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "TransientViews::descentView", Target: "Transient::Descent",
	})
	session := snap.Session
	wantStrings(t, "initial active states", snap.ActiveStates, []string{id["coasting"]})
	mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{id["braking"]}})

	snap = mustDebug(t, s, MethodDebugAdvance, advanceBy(session, 20))
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != id["braking"] || !strings.Contains(snap.Reason, "breakpoint braking") {
		t.Errorf("pausedAt = %q reason = %q, want the breakpoint on braking", snap.PausedAt, snap.Reason)
	}
	wantStrings(t, "active at the breakpoint", snap.ActiveStates, []string{id["braking"]})
	wantStrings(t, "taken to the breakpoint", edges(snap.Taken), []string{id["coasting"] + "->" + id["braking"]})
	if snap.Time != 5 {
		t.Errorf("time at the breakpoint = %v, want 5: the advance stops where it paused", snap.Time)
	}

	// Resuming leaves the transient state at the held instant; the advance
	// then finishes without pausing again.
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != "" {
		t.Errorf("pausedAt = %q after resuming, want none", snap.PausedAt)
	}
	wantStrings(t, "active after resuming", snap.ActiveStates, []string{id["landed"]})
	wantStrings(t, "taken after resuming", edges(snap.Taken), []string{id["braking"] + "->" + id["landed"]})
	if snap.Time != 5 {
		t.Errorf("time after resuming = %v, want 5", snap.Time)
	}
	snap = mustDebug(t, s, MethodDebugAdvance, advanceBy(session, 15))
	if snap.Time != 20 || snap.PausedAt != "" {
		t.Errorf("after the rest of the advance: time = %v pausedAt = %q, want 20 and none", snap.Time, snap.PausedAt)
	}
}

// debugJunction is a machine whose signal routes through a junction deciding on
// a counter.
const debugJunction = `package Routing {
	private import ScalarValues::*;
	state def Ops {
		attribute count : Integer = 0;
		entry; then idle;
		state idle;
		junction route;
		state low;
		state high;
		transition first idle accept Go do assign count := count + 1 then route;
		transition first route if count > 5 then high;
		transition first route then low;
	}
	attribute def Go;
}
package RoutingViews {
	private import StandardViewDefinitions::*;
	view opsView : StateTransitionView { expose Routing::Ops; }
}
`

// debugSameNamedSignals declares Go twice, the machine's own after another
// package's, and one signal inside the machine's body.
const debugSameNamedSignals = `package A {
	attribute def Go;
}
package B {
	attribute def Go;
	state def Machine {
		attribute def Local;
		entry; then idle;
		state idle;
		state going;
		state done;
		transition first idle accept Go then going;
		transition first going accept Local then done;
	}
}
package BViews {
	private import StandardViewDefinitions::*;
	view machineView : StateTransitionView { expose B::Machine; }
}
`

// A signal name resolves from the target's scope, as an accept written there
// does: the machine's own Go rather than the first Go declared in the document,
// and a signal declared in its body; a qualified name still names what it says.
func TestDebugSendResolvesSignalInTargetScope(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/s.sysml", debugSameNamedSignals)
	id := ids(t, render(t, s, docURI, "BViews::machineView"))
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "BViews::machineView", Target: "B::Machine",
	})
	session := snap.Session

	if _, err := debugCall(t, s, MethodDebugSend, &debugSendParams{Session: session, Signal: "A::Go"}); !errors.Is(err, ErrDebugSignal) {
		t.Errorf("send of A::Go: err = %v, want %v", err, ErrDebugSignal)
	}
	snap = mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: session, Signal: "Go"})
	if len(snap.Queue) != 1 || snap.Queue[0].Event != "Go" || !snap.Queue[0].Pending {
		t.Errorf("queue after sending Go = %+v, want the pending Go", snap.Queue)
	}
	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session})
	wantStrings(t, "active after Go", snap.ActiveStates, []string{id["going"]})

	snap = mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: session, Signal: "Local"})
	if len(snap.Queue) != 1 || snap.Queue[0].Event != "Local" {
		t.Errorf("queue after sending Local = %+v, want the pending Local", snap.Queue)
	}
	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session})
	wantStrings(t, "active after Local", snap.ActiveStates, []string{id["done"]})

	if _, err := debugCall(t, s, MethodDebugSend, &debugSendParams{Session: session, Signal: "Go(1)"}); !errors.Is(err, ErrDebugSignal) {
		t.Errorf("send of an expression: err = %v, want %v", err, ErrDebugSignal)
	}
}

// A breakpoint on a pseudostate pauses the run once the dispatch routed through
// it completes, the pseudostate reported as where it paused.
func TestDebugBreakpointOnAPseudostatePauses(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/j.sysml", debugJunction)
	id := ids(t, render(t, s, docURI, "RoutingViews::opsView"))
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "RoutingViews::opsView", Target: "Routing::Ops",
	})
	session := snap.Session
	snap = mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{id["route"]}})
	wantStrings(t, "breakpoints", snap.Breakpoints, []string{id["route"]})

	mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: session, Signal: "Routing::Go"})
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != id["route"] || !strings.Contains(snap.Reason, "breakpoint route") {
		t.Errorf("pausedAt = %q reason = %q, want the breakpoint on route", snap.PausedAt, snap.Reason)
	}
	wantStrings(t, "active at the breakpoint", snap.ActiveStates, []string{id["low"]})
	wantStrings(t, "taken to the breakpoint", edges(snap.Taken), []string{id["idle"] + "->" + id["route"], id["route"] + "->" + id["low"]})

	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
	if snap.PausedAt != "" {
		t.Errorf("pausedAt = %q after resuming, want none", snap.PausedAt)
	}
}

// A breakpoint on the machine's `done` pauses the run standing on it, before the
// machine completes; the next step completes it.
func TestDebugBreakpointOnDonePausesBeforeCompletion(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/h.sysml", `package Halting {
	attribute def Halt;
	state def Runner {
		entry; then idle;
		state idle;
		transition first idle accept Halt then done;
	}
}
package HaltingViews {
	private import StandardViewDefinitions::*;
	view runnerView : StateTransitionView { expose Halting::Runner; }
}
`)
	id := ids(t, render(t, s, docURI, "HaltingViews::runnerView"))
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "HaltingViews::runnerView", Target: "Halting::Runner",
	})
	session := snap.Session
	snap = mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{id["done"]}})
	wantStrings(t, "breakpoints", snap.Breakpoints, []string{id["done"]})

	mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: session, Signal: "Halting::Halt"})
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != id["done"] || !strings.Contains(snap.Reason, "breakpoint done") {
		t.Errorf("pausedAt = %q reason = %q, want the breakpoint on done", snap.PausedAt, snap.Reason)
	}
	wantStrings(t, "active at the breakpoint", snap.ActiveStates, []string{id["done"]})
	wantStrings(t, "taken to the breakpoint", edges(snap.Taken), []string{id["idle"] + "->" + id["done"]})

	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session})
	wantState(t, snap, debugCompleted)
	if snap.PausedAt != "" {
		t.Errorf("pausedAt = %q after completing, want none", snap.PausedAt)
	}

	// Without the breakpoint the same run completes in one continue, standing
	// where the paused run ended up.
	plain := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "HaltingViews::runnerView", Target: "Halting::Runner",
	})
	mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: plain.Session, Signal: "Halting::Halt"})
	plain = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: plain.Session})
	wantState(t, plain, debugCompleted)
	wantStrings(t, "active after completing without the breakpoint", plain.ActiveStates, snap.ActiveStates)
}

// A step that lands a token on an action breakpoint pauses there, as a continue
// does; the step after moves it past the breakpoint.
func TestDebugStepPausesAtAnActionBreakpoint(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/f.sysml", debugFlow)
	id := ids(t, render(t, s, docURI, "FlowViews::driveView"))
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "FlowViews::driveView", Target: "Flows::Drive",
	})
	session := snap.Session
	mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{id["split"]}})

	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != id["split"] || !strings.Contains(snap.Reason, "breakpoint split") {
		t.Errorf("pausedAt = %q reason = %q, want the breakpoint on split", snap.PausedAt, snap.Reason)
	}
	if len(snap.Tokens) != 1 || snap.Tokens[0].Node != id["split"] {
		t.Errorf("tokens = %+v, want the one held at split", snap.Tokens)
	}
	wantStrings(t, "taken to the breakpoint", edges(snap.Taken), []string{id["start"] + "->" + id["split"]})

	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session})
	wantState(t, snap, debugRunning)
	if snap.PausedAt != "" {
		t.Errorf("pausedAt = %q after resuming, want none", snap.PausedAt)
	}
	if len(snap.Tokens) != 4 || debugTokenAt(t, snap, id["tally"]).Node != id["tally"] {
		t.Errorf("tokens = %+v, want the fork's four branches", snap.Tokens)
	}
}

// pausedAtSplit starts a session on the drive flow and steps it to a breakpoint
// on its fork, returning the session and the rendering's IDs.
func pausedAtSplit(t *testing.T) (*Server, string, map[string]string) {
	t.Helper()
	s, docURI, _ := debugServer(t, "/w/f.sysml", debugFlow)
	id := ids(t, render(t, s, docURI, "FlowViews::driveView"))
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "FlowViews::driveView", Target: "Flows::Drive",
	})
	session := snap.Session
	mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{id["split"]}})
	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != id["split"] {
		t.Fatalf("pausedAt = %q, want the breakpoint on split", snap.PausedAt)
	}
	return s, session, id
}

// Setting the breakpoints again while paused at one keeps the pause made: the
// next step moves past it rather than pausing at the same node again. Removing
// the breakpoint and setting it again does pause the run there again.
func TestDebugBreakpointsSetAgainWhilePausedKeepThePause(t *testing.T) {
	s, session, id := pausedAtSplit(t)
	snap := mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{id["split"]}})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != id["split"] {
		t.Errorf("pausedAt = %q after setting the same breakpoints, want still split", snap.PausedAt)
	}
	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session})
	wantState(t, snap, debugRunning)
	if snap.PausedAt != "" {
		t.Errorf("pausedAt = %q after the step, want the run past split", snap.PausedAt)
	}
	if len(snap.Tokens) != 4 || debugTokenAt(t, snap, id["tally"]).Node != id["tally"] {
		t.Errorf("tokens = %+v, want the fork's four branches", snap.Tokens)
	}

	s, session, id = pausedAtSplit(t)
	snap = mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session})
	if len(snap.Breakpoints) != 0 {
		t.Errorf("breakpoints = %v after removing the breakpoint, want none", snap.Breakpoints)
	}
	mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{id["split"]}})
	snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != id["split"] || len(snap.Tokens) != 1 || snap.Tokens[0].Node != id["split"] {
		t.Errorf("pausedAt = %q tokens = %+v after re-setting the breakpoint, want the token held at split again", snap.PausedAt, snap.Tokens)
	}
}

// An advance resumes an action paused at a breakpoint: the clock moves, and the
// run goes past the breakpoint to what is due by then.
func TestDebugAdvanceResumesAPausedAction(t *testing.T) {
	s, session, id := pausedAtSplit(t)
	snap := mustDebug(t, s, MethodDebugAdvance, advanceBy(session, 5))
	if snap.Time != 5 {
		t.Errorf("time after advance = %v, want 5", snap.Time)
	}
	wantState(t, snap, debugWaiting)
	if snap.PausedAt != "" {
		t.Errorf("pausedAt = %q after the advance, want the run past split", snap.PausedAt)
	}
	for _, tok := range snap.Tokens {
		if tok.Node == id["split"] {
			t.Errorf("tokens = %+v, want none still held at split", snap.Tokens)
		}
	}
	if debugTokenAt(t, snap, id["listen"]).Node != id["listen"] {
		t.Errorf("tokens = %+v, want one waiting at listen", snap.Tokens)
	}
	taken := edges(snap.Taken)
	for _, want := range []string{id["split"] + "->" + id["pause"], id["pause"] + "->" + id["sync"]} {
		if !slices.Contains(taken, want) {
			t.Errorf("taken = %v, want %s among them", taken, want)
		}
	}
}

// debugReused is a flow whose two nested actions each run the one action they
// inherit, so its rendering draws that declaration twice.
const debugReused = `package Reused {
	action def Check {
		action look;
	}
	action def Twice {
		first start;
		action a : Check { first begin; then look; }
		action b : Check { first begin; then look; }
		done;
		succession first start then a;
		succession first a then b;
		succession first b then done;
	}
}
package ReusedViews {
	private import StandardViewDefinitions::*;
	view twiceView : ActionFlowView { expose Reused::Twice; }
}
`

// A breakpoint on one drawing of a node a nested flow reuses stops the run at
// that occurrence only, not at the same declaration another nested flow runs.
func TestDebugBreakpointOnAReusedNodeStopsAtItsOccurrence(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/r.sysml", debugReused)
	r := render(t, s, docURI, "ReusedViews::twiceView")
	var a, b, lookInA, lookInB string
	for _, n := range r.Nodes {
		switch n.Name {
		case "a":
			a = n.ID
		case "b":
			b = n.ID
		}
	}
	for _, n := range r.Nodes {
		if n.Name != "look" {
			continue
		}
		switch n.Parent {
		case a:
			lookInA = n.ID
		case b:
			lookInB = n.ID
		}
	}
	if lookInA == "" || lookInB == "" || lookInA == lookInB {
		t.Fatalf("look is drawn as %q in a and %q in b, want one drawing in each", lookInA, lookInB)
	}

	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "ReusedViews::twiceView", Target: "Reused::Twice",
	})
	session := snap.Session
	snap = mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{lookInB}})
	wantStrings(t, "breakpoints", snap.Breakpoints, []string{lookInB})
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != lookInB || !strings.Contains(snap.Reason, "breakpoint look") {
		t.Errorf("pausedAt = %q reason = %q, want the breakpoint on look within b", snap.PausedAt, snap.Reason)
	}
	if len(snap.Tokens) != 1 || snap.Tokens[0].Node != lookInB {
		t.Errorf("tokens = %+v, want the one held at look within b", snap.Tokens)
	}
	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
	wantState(t, snap, debugCompleted)
}

// The node paused at is the breakpoint the run stopped at, not the first token
// found on a breakpoint: a token still held at one already stopped at does not
// stand in for the one the run has just reached.
func TestDebugPausedAtIsTheBreakpointReachedNotTheFirstTokenOnOne(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/f.sysml", debugFlow)
	id := ids(t, render(t, s, docURI, "FlowViews::driveView"))
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "FlowViews::driveView", Target: "Flows::Drive",
	})
	session := snap.Session
	mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{id["pause"], id["listen"]}})

	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != id["pause"] || !strings.Contains(snap.Reason, "breakpoint pause") {
		t.Fatalf("pausedAt = %q reason = %q, want the breakpoint on pause", snap.PausedAt, snap.Reason)
	}

	snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
	wantState(t, snap, debugSuspended)
	if snap.PausedAt != id["listen"] || !strings.Contains(snap.Reason, "breakpoint listen") {
		t.Errorf("pausedAt = %q reason = %q, want the breakpoint on listen", snap.PausedAt, snap.Reason)
	}
	var at []string
	for _, tok := range snap.Tokens {
		at = append(at, tok.Node)
	}
	if !slices.Contains(at, id["pause"]) || !slices.Contains(at, id["listen"]) {
		t.Errorf("tokens at %v, want ones still held at pause and at listen", at)
	}
}

// debugCounter is a robot whose machine counts the timer firing; guards then
// tell one firing from two.
const debugCounter = `package Counting {
	private import ScalarValues::*;
	private import SI::*;
	part def Robot {
		attribute count : Integer = 0;
		exhibit state ops {
			entry; then waiting;
			state waiting;
			state elapsed;
			state once;
			state twice;
			transition first waiting accept after 5 [s] do assign count := count + 1 then elapsed;
			transition first elapsed accept after 1 [s] if count == 1 then once;
			transition first elapsed accept after 1 [s] if count > 1 then twice;
		}
	}
	state def Ops { entry; then idle; state idle; }
	part def Twin {
		exhibit state left : Ops;
		exhibit state right : Ops;
	}
}
package CountingViews {
	private import StandardViewDefinitions::*;
	view opsView : StateTransitionView { expose Counting::Robot::ops; }
	view twinView : StateTransitionView { expose Counting::Ops; }
}
`

// A session on an object exhibiting the target debugs the machine the object
// already runs, so its timed effect happens once; an object exhibiting it twice
// is ambiguous.
func TestDebugStartAttachesToThePerformersMachine(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/c.sysml", debugCounter)
	id := ids(t, render(t, s, docURI, "CountingViews::opsView"))
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "CountingViews::opsView", Target: "Counting::Robot::ops", Object: "Counting::Robot",
	})
	session := snap.Session
	wantStrings(t, "initial active states", snap.ActiveStates, []string{id["waiting"]})
	if len(snap.Queue) != 1 {
		t.Errorf("queue = %+v, want the one timer of the one machine", snap.Queue)
	}
	snap = mustDebug(t, s, MethodDebugAdvance, advanceBy(session, 6))
	wantStrings(t, "active after both timers", snap.ActiveStates, []string{id["once"]})
	wantStrings(t, "taken", edges(snap.Taken), []string{id["waiting"] + "->" + id["elapsed"], id["elapsed"] + "->" + id["once"]})

	_, err := debugCall(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "CountingViews::twinView", Target: "Counting::Ops", Object: "Counting::Twin",
	})
	if err == nil || !strings.Contains(err.Error(), "2 times") {
		t.Errorf("start on an object exhibiting the machine twice: err = %v, want the ambiguity refused", err)
	}
}

// A session's executors keep only the firings and successions since the mark
// the last snapshot read from: each snapshot releases what the one before it
// reported, so a long-lived session's record stays bounded while the counts go on.
func TestDebugSessionReleasesTheHistoryItReports(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/m.sysml", debugMachine)
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Robot",
	})
	session := snap.Session
	s.debug.mu.Lock()
	machine := s.debug.sessions[session].machine
	s.debug.mu.Unlock()
	if len(snap.Taken) == 0 || len(machine.FiredTransitions()) != len(snap.Taken) || machine.FiredCount() != len(snap.Taken) {
		t.Fatalf("after start taken %d, kept %d of %d; want the entry transitions reported and kept", len(snap.Taken), len(machine.FiredTransitions()), machine.FiredCount())
	}
	fired := machine.FiredCount()
	snap = mustDebug(t, s, MethodDebugAdvance, advanceBy(session, 6))
	if len(snap.Taken) == 0 || len(machine.FiredTransitions()) != len(snap.Taken) || machine.FiredCount() != fired+len(snap.Taken) {
		t.Errorf("after advance taken %d, kept %d of %d; want the timer's transition reported and kept alone, %d counted", len(snap.Taken), len(machine.FiredTransitions()), machine.FiredCount(), fired+len(snap.Taken))
	}
	snap = mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session})
	if len(snap.Taken) != 0 || len(machine.FiredTransitions()) != 0 || machine.FiredCount() != fired+1 {
		t.Errorf("after a request firing nothing taken %d, kept %d of %d; want none, none kept, %d counted", len(snap.Taken), len(machine.FiredTransitions()), machine.FiredCount(), fired+1)
	}

	s, docURI, _ = debugServer(t, "/w/f.sysml", debugFlow)
	snap = mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "FlowViews::driveView", Target: "Flows::Drive",
	})
	session = snap.Session
	s.debug.mu.Lock()
	action := s.debug.sessions[session].action
	s.debug.mu.Unlock()
	var taken int
	for range 3 {
		snap = mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session})
		taken += len(snap.Taken)
		if len(snap.Taken) == 0 || len(action.Traversals()) != len(snap.Taken) || action.TraversalCount() != taken {
			t.Fatalf("after a step taken %d, kept %d of %d; want the step's successions reported and kept alone, %d counted", len(snap.Taken), len(action.Traversals()), action.TraversalCount(), taken)
		}
	}
}

// A machine's queue shows every message pending in its context, a message no
// state of the machine accepts now included: it is in flight all the same.
func TestDebugMachineQueueShowsEveryPendingMessage(t *testing.T) {
	s, docURI, _ := debugServer(t, "/w/m.sysml", debugMachine)
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Robot",
	})
	session := snap.Session
	if _, err := debugCall(t, s, MethodDebugSend, &debugSendParams{Session: session, Signal: "Halt"}); !errors.Is(err, ErrDebugSignal) {
		t.Fatalf("send Halt to a machine in idle: err = %v, want it refused as not accepted now", err)
	}
	s.debug.mu.Lock()
	s.debug.sessions[session].rt.PostMessage(runtime.NamedSignalMessage("Halt", nil))
	s.debug.mu.Unlock()
	snap = mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session})
	var pending []string
	for _, event := range snap.Queue {
		if event.Pending {
			pending = append(pending, event.Event)
		}
	}
	wantStrings(t, "pending messages", pending, []string{"Halt"})
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
	snap = mustDebug(t, s, MethodDebugAdvance, advanceBy(snap.Session, 5))
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

// An edit reaches the workspace under the lock requests hold, so a request in
// flight finishes at the documents it began with and the next one finds the
// session already moved to the edited ones.
func TestDebugEditWaitsForTheRequestInFlight(t *testing.T) {
	s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
	name := uriToName(docURI)
	session := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Robot",
	}).Session

	// Held as a request holds it while it drives the session.
	s.debug.mu.Lock()
	applied := make(chan struct{})
	go func() {
		defer close(applied)
		s.applyDidChange(context.Background(), name, []rawContentChange{{Text: "package Extra {\n\tpart def Spare;\n}\n" + debugMachine}}, 2)
	}()
	time.Sleep(50 * time.Millisecond)
	if v := s.ws.Document(name).Version; v != 1 {
		t.Fatalf("the edit reached the workspace at version %d while a request held the session", v)
	}
	if v := s.debug.sessions[session].version; v != 1 {
		t.Fatalf("the session moved to version %d while a request held it", v)
	}
	s.debug.mu.Unlock()
	<-applied

	changed := rec.debugChanged()
	if len(changed) != 1 || changed[0].Version != 2 {
		t.Fatalf("debugChanged = %d notifications, want one at version 2", len(changed))
	}
	if snap := mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session}); snap.Version != 2 {
		t.Errorf("the step after the edit answered version %d, want 2", snap.Version)
	}
}

// Every snapshot of a session, answered or notified, takes the next revision, so
// a client can tell the newer of two that arrive out of order.
func TestDebugSnapshotsAreNumbered(t *testing.T) {
	s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Robot",
	})
	session := snap.Session
	if snap.Revision != 1 {
		t.Errorf("the start answered revision %d, want 1", snap.Revision)
	}
	last := snap.Revision
	next := func(what string, snap *debugSnapshot) {
		t.Helper()
		if snap.Revision != last+1 {
			t.Errorf("%s: revision %d follows %d, want %d", what, snap.Revision, last, last+1)
		}
		last = snap.Revision
	}
	next("step", mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: session}))
	next("breakpoints", mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session}))

	s.applyDidChange(context.Background(), uriToName(docURI), []rawContentChange{{Text: "package Extra {\n\tpart def Spare;\n}\n" + debugMachine}}, 2)
	changed := rec.debugChanged()
	if len(changed) != 1 {
		t.Fatalf("debugChanged = %v, want one moved snapshot", changed)
	}
	next("moved", changed[0])
	next("continue", mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session}))

	mustDebug(t, s, MethodDebugStop, &debugSessionParams{Session: session})
	another := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Robot",
	})
	if another.Session == session || another.Revision != 1 {
		t.Errorf("a new session %s starts at revision %d, want its own count from 1", another.Session, another.Revision)
	}
}

// A session paused at a breakpoint when an unrelated edit redraws the view stays
// paused, and the moved snapshot names the breakpoint node in the fresh IDs, for
// a machine and for an action paused in a nested flow alike.
func TestDebugPauseFollowsEdits(t *testing.T) {
	t.Run("machine", func(t *testing.T) {
		s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
		id := ids(t, render(t, s, docURI, "#state"))
		snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         "#state", Target: "Machines::Ops", Object: "Machines::Robot",
		})
		session := snap.Session
		mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{id["busy"]}})
		mustDebug(t, s, MethodDebugSend, &debugSendParams{Session: session, Signal: "Go", Args: map[string]string{"level": "2"}})
		snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
		wantState(t, snap, debugSuspended)
		if snap.PausedAt != id["busy"] {
			t.Fatalf("pausedAt = %q, want the breakpoint on busy", snap.PausedAt)
		}

		// A machine declared above Ops is drawn before it, moving every ID.
		s.applyDidChange(context.Background(), uriToName(docURI), []rawContentChange{{Text: "package Extra {\n\tstate def Spare { state one; }\n}\n" + debugMachine}}, 2)
		fresh := ids(t, render(t, s, docURI, "#state"))
		if fresh["busy"] == id["busy"] {
			t.Fatalf("the edit left busy at %s; the fixture must move it", id["busy"])
		}
		changed := rec.debugChanged()
		if len(changed) != 1 {
			t.Fatalf("debugChanged = %v, want one moved snapshot", changed)
		}
		moved := changed[0]
		wantState(t, moved, debugSuspended)
		if moved.PausedAt != fresh["busy"] || !strings.Contains(moved.Reason, "breakpoint busy") {
			t.Errorf("moved pausedAt = %q reason = %q, want %s, the breakpoint on busy", moved.PausedAt, moved.Reason, fresh["busy"])
		}
		wantStrings(t, "moved breakpoints", moved.Breakpoints, []string{fresh["busy"]})

		snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
		if snap.PausedAt != "" {
			t.Errorf("pausedAt = %q after resuming, want none", snap.PausedAt)
		}
		wantStrings(t, "active after resuming", snap.ActiveStates, []string{fresh["motion"], fresh["busy"], fresh["working"], fresh["clock"], fresh["waiting"]})
	})
	t.Run("action", func(t *testing.T) {
		s, docURI, rec := debugServer(t, "/w/f.sysml", debugFlow)
		id := ids(t, render(t, s, docURI, "#action"))
		snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "#action", Target: "Flows::Drive",
		})
		session := snap.Session
		mustDebug(t, s, MethodDebugBreakpoints, &debugBreakpointsParams{Session: session, NodeIDs: []string{id["warm"]}})
		snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
		wantState(t, snap, debugSuspended)
		if snap.PausedAt != id["warm"] {
			t.Fatalf("pausedAt = %q, want the breakpoint on warm", snap.PausedAt)
		}

		s.applyDidChange(context.Background(), uriToName(docURI), []rawContentChange{{Text: "package Extra {\n\taction def Spare { action one; }\n}\n" + debugFlow}}, 2)
		fresh := ids(t, render(t, s, docURI, "#action"))
		if fresh["warm"] == id["warm"] {
			t.Fatalf("the edit left warm at %s; the fixture must move it", id["warm"])
		}
		changed := rec.debugChanged()
		if len(changed) != 1 {
			t.Fatalf("debugChanged = %v, want one moved snapshot", changed)
		}
		moved := changed[0]
		wantState(t, moved, debugSuspended)
		if moved.PausedAt != fresh["warm"] || !strings.Contains(moved.Reason, "breakpoint") {
			t.Errorf("moved pausedAt = %q reason = %q, want %s, the breakpoint on warm", moved.PausedAt, moved.Reason, fresh["warm"])
		}
		if debugTokenAt(t, moved, fresh["warm"]).Node != fresh["warm"] {
			t.Errorf("moved tokens = %+v, want one held at warm", moved.Tokens)
		}

		snap = mustDebug(t, s, MethodDebugContinue, &debugSessionParams{Session: session})
		if snap.PausedAt != "" {
			t.Errorf("pausedAt = %q after resuming, want none", snap.PausedAt)
		}
	})
}

// A start reads the workspace once and registers the session after; an edit
// landing in between is not lost. One leaving the behavior as it was moves the
// session to the fresh IDs before it answers; one to the behavior — of the same
// length, so every span stays put — refuses the start.
func TestDebugStartSeesAnEditSinceItsReading(t *testing.T) {
	s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
	ctx := context.Background()
	name := uriToName(docURI)
	params := &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Machines::Robot",
	}

	sess, err := s.debugPrepare(params)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	shifted := "package Extra {\n\tpart def Spare;\n}\n" + debugMachine
	s.applyDidChange(ctx, name, []rawContentChange{{Text: shifted}}, 2)
	if changed := rec.debugChanged(); len(changed) != 0 {
		t.Fatalf("debugChanged for a session not yet registered: %v", changed)
	}
	snap, err := s.debugRegister(sess)
	if err != nil {
		t.Fatalf("register after an unrelated edit: %v", err)
	}
	if snap.Version != 2 {
		t.Errorf("snapshot is at version %d, want 2", snap.Version)
	}
	fresh := ids(t, render(t, s, docURI, "MachineViews::opsView"))
	wantStrings(t, "active states", snap.ActiveStates, []string{fresh["motion"], fresh["idle"], fresh["clock"], fresh["waiting"]})
	// The move is reported by the first snapshot, which is the one a start
	// without the edit would answer: the same revision, entry edges and notes.
	undisturbed := mustDebug(t, s, MethodDebugStart, params)
	if snap.Revision != 1 || undisturbed.Revision != 1 {
		t.Errorf("revisions = %d after the edit, %d without, want 1 for a first snapshot", snap.Revision, undisturbed.Revision)
	}
	if len(undisturbed.Taken) == 0 {
		t.Fatalf("an undisturbed start takes no entry edges: %+v", undisturbed)
	}
	if !reflect.DeepEqual(snap.Taken, undisturbed.Taken) {
		t.Errorf("taken = %+v after the edit, want the entry edges an undisturbed start reports: %+v", snap.Taken, undisturbed.Taken)
	}
	if !reflect.DeepEqual(snap.Notes, undisturbed.Notes) {
		t.Errorf("notes = %q after the edit, want an undisturbed start's %q", snap.Notes, undisturbed.Notes)
	}
	mustDebug(t, s, MethodDebugStop, &debugSessionParams{Session: undisturbed.Session})
	stepped := mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: snap.Session})
	wantStrings(t, "active states after a step", stepped.ActiveStates, []string{fresh["motion"], fresh["idle"], fresh["clock"], fresh["elapsed"]})

	sess, err = s.debugPrepare(params)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	s.applyDidChange(ctx, name, []rawContentChange{{Text: strings.Replace(shifted, "state idle;", "state lazy;", 1)}}, 3)
	if changed := rec.debugChanged(); len(changed) != 1 || changed[0].Session != snap.Session || changed[0].State != debugEnded {
		t.Fatalf("debugChanged after editing the machine = %v, want the end of %s", changed, snap.Session)
	}
	if _, err := s.debugRegister(sess); !errors.Is(err, ErrDebugTarget) || !strings.Contains(err.Error(), "Machines::Ops was edited") {
		t.Errorf("register after the machine was edited: err = %v, want %v saying Machines::Ops was edited", err, ErrDebugTarget)
	}
	s.debug.mu.Lock()
	live := len(s.debug.sessions)
	s.debug.mu.Unlock()
	if live != 0 {
		t.Errorf("%d sessions live after the refused start, want none", live)
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
	t.Run("view rewritten but still drawing the target", func(t *testing.T) {
		s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
		session := start(t, s, docURI)
		s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: strings.Replace(debugMachine, "{ expose Machines::Ops; }", "{ expose Machines::Ops; expose Machines::Robot; }", 1)}}, 2)
		if reason := lastEnd(t, rec, session).Reason; !strings.Contains(reason, "MachineViews::opsView was edited") {
			t.Errorf("reason = %q", reason)
		}
	})
	t.Run("pseudo-view outlives edits around the target", func(t *testing.T) {
		s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
		snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "#state:Machines::Ops", Target: "Machines::Ops",
		})
		s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: strings.Replace(debugMachine, "attribute def Halt;", "attribute def Halt;\n\tattribute def Resume;", 1)}}, 2)
		changed := rec.debugChanged()
		if len(changed) != 1 || changed[0].Session != snap.Session || changed[0].State == debugEnded || changed[0].Version != 2 {
			t.Fatalf("debugChanged after an edit around a pseudo-view's target = %v", changed)
		}
		s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: strings.Replace(debugMachine, "state elapsed;", "state elapsed;\n\t\t\tstate late;", 1)}}, 3)
		if reason := lastEnd(t, rec, snap.Session).Reason; !strings.Contains(reason, "Machines::Ops was edited") {
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
	t.Run("document closed with its text still on disk", func(t *testing.T) {
		name := filepath.Join(t.TempDir(), "m.sysml")
		if err := os.WriteFile(name, []byte(debugMachine), 0o600); err != nil {
			t.Fatal(err)
		}
		s, docURI, rec := debugServer(t, name, debugMachine)
		session := start(t, s, docURI)
		if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: docURI}}); err != nil {
			t.Fatal(err)
		}
		if s.ws.Document(name) == nil {
			t.Fatal("closing dropped the document the workspace folder holds")
		}
		changed := rec.debugChanged()
		if len(changed) != 1 {
			t.Fatalf("debugChanged = %d notifications, want the one ending the session", len(changed))
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

// debugDerived draws behaviors whose content comes from the definitions they
// specialize, declared apart from them.
const debugDerived = `package Inherited {
	private import ScalarValues::*;
	attribute def Go;
	state def Base {
		entry; then idle;
		state idle;
		accept Go then done;
		state done;
	}
	action def BaseFlow {
		action a;
		action b;
	}
	package Sub {
		state def Derived :> Base;
		state def Typing {
			entry; then run;
			state run : Base;
		}
		action def DerivedFlow :> BaseFlow {
			first start;
			then a;
			succession first a then b;
			then done;
		}
	}
}
package InheritedViews {
	private import StandardViewDefinitions::*;
	view derivedView : StateTransitionView { expose Inherited::Sub::Derived; }
	view typingView : StateTransitionView { expose Inherited::Sub::Typing; }
	view flowView : ActionFlowView { expose Inherited::Sub::DerivedFlow; }
}
`

// An edit to a declaration the target takes its content from — not the target's
// own text — ends the session: the runtime no longer stands for what is drawn.
func TestDebugSessionEndsWhenInheritedContentChanges(t *testing.T) {
	ctx := context.Background()
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
	cases := []struct {
		name, view, target, old, new, reason string
	}{
		{"state definition specialized", "InheritedViews::derivedView", "Inherited::Sub::Derived",
			"state done;", "state done;\n\t\tstate cooling;", "Inherited::Base was edited"},
		{"state definition specialized is removed", "InheritedViews::derivedView", "Inherited::Sub::Derived",
			"state def Base {", "state def Root {", "Inherited::Base is no longer declared"},
		{"state definition specialized is shadowed", "InheritedViews::derivedView", "Inherited::Sub::Derived",
			"state def Derived :> Base;", "state def Base { entry; then off; state off; }\n\t\tstate def Derived :> Base;",
			"Inherited::Sub::Derived now reads Inherited::Sub::Base"},
		{"state definition typing a nested state", "InheritedViews::typingView", "Inherited::Sub::Typing",
			"state done;", "state done;\n\t\tstate cooling;", "Inherited::Base was edited"},
		{"action definition specialized", "InheritedViews::flowView", "Inherited::Sub::DerivedFlow",
			"action b;", "action b { attribute n : Integer; }", "Inherited::BaseFlow was edited"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, docURI, rec := debugServer(t, "/w/i.sysml", debugDerived)
			snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: tc.view, Target: tc.target,
			})
			if strings.Count(debugDerived, tc.old) != 1 {
				t.Fatalf("fixture writes %q %d times", tc.old, strings.Count(debugDerived, tc.old))
			}
			s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: strings.Replace(debugDerived, tc.old, tc.new, 1)}}, 2)
			if reason := lastEnd(t, rec, snap.Session).Reason; !strings.Contains(reason, tc.reason) {
				t.Errorf("reason = %q, want it to name %q", reason, tc.reason)
			}
		})
	}

	// An edit elsewhere in the definition's package leaves the session running.
	s, docURI, rec := debugServer(t, "/w/i.sysml", debugDerived)
	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "InheritedViews::derivedView", Target: "Inherited::Sub::Derived",
	})
	s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: strings.Replace(debugDerived, "attribute def Go;", "attribute def Go;\n\tattribute def Stop;", 1)}}, 2)
	changed := rec.debugChanged()
	if len(changed) != 1 || changed[0].Session != snap.Session || changed[0].State == debugEnded {
		t.Fatalf("debugChanged after an unrelated edit = %v", changed)
	}
}

// A view debugs the behavior it exposes from another document, performed by an
// object from a third; edits there move or end the session, duplicates are refused.
func TestDebugTargetDeclaredInAnotherDocument(t *testing.T) {
	ctx := context.Background()
	split := strings.Index(debugMachine, "package MachineViews")
	machines, views := debugMachine[:split], debugMachine[split:]
	open := func(t *testing.T, s *Server, name, src string) uri.URI {
		t.Helper()
		docURI := uri.File(name)
		if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{URI: docURI, LanguageID: "sysml", Version: 1, Text: src},
		}); err != nil {
			t.Fatalf("DidOpen %s: %v", name, err)
		}
		return docURI
	}
	s, viewsURI, rec := debugServer(t, "/w/views.sysml", views)
	machinesURI := open(t, s, "/w/machines.sysml", machines)
	open(t, s, "/w/robots.sysml", "package Robots { part def Rover { exhibit state ops : Machines::Ops; } }\n")
	r := render(t, s, viewsURI, "MachineViews::opsView")
	id := ids(t, r)

	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: viewsURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Robots::Rover",
	})
	if snap.Target != "Machines::Ops" || snap.Object != "Robots::Rover" || snap.Root != r.Nodes[0].ID {
		t.Fatalf("start snapshot = %s", describe(snap))
	}
	wantState(t, snap, debugWaiting)
	wantStrings(t, "initial active states", snap.ActiveStates, []string{id["motion"], id["idle"], id["clock"], id["waiting"]})

	// An edit to the machine's document that leaves the machine as declared
	// keeps the session running; one rewriting the machine ends it.
	s.applyDidChange(ctx, uriToName(machinesURI), []rawContentChange{{Text: strings.Replace(machines, "attribute def Halt;", "attribute def Halt;\n\tattribute def Stop;", 1)}}, 2)
	for _, moved := range rec.debugChanged() {
		if moved.Session != snap.Session || moved.State == debugEnded {
			t.Fatalf("debugChanged after an unrelated edit to the machine's document = %s", describe(moved))
		}
		wantStrings(t, "active states after an unrelated edit", moved.ActiveStates, snap.ActiveStates)
	}
	s.applyDidChange(ctx, uriToName(machinesURI), []rawContentChange{{Text: strings.Replace(machines, "state elapsed;", "state elapsed;\n\t\tstate cooling;", 1)}}, 3)
	changed := rec.debugChanged()
	last := changed[len(changed)-1]
	if last.Session != snap.Session || last.State != debugEnded || !strings.Contains(last.Reason, "Machines::Ops was edited") {
		t.Fatalf("debugChanged after the machine was edited = %s", describe(last))
	}

	// Closing the machine's document ends a session on it too.
	snap = mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: viewsURI}, View: "MachineViews::opsView", Target: "Machines::Ops",
	})
	if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: machinesURI}}); err != nil {
		t.Fatalf("DidClose: %v", err)
	}
	changed = rec.debugChanged()
	last = changed[len(changed)-1]
	if last.Session != snap.Session || last.State != debugEnded || !strings.Contains(last.Reason, "machines.sysml was closed") {
		t.Fatalf("debugChanged after closing the machine's document = %s", describe(last))
	}

	// The same name declared by two documents names neither.
	open(t, s, "/w/machines.sysml", machines)
	open(t, s, "/w/spare.sysml", machines)
	_, err := s.DebugStart(&debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: viewsURI}, View: "MachineViews::opsView", Target: "Machines::Ops",
	})
	if !errors.Is(err, ErrDebugTarget) || !strings.Contains(err.Error(), "/w/machines.sysml, /w/spare.sysml") {
		t.Fatalf("start of an ambiguous target: err = %v, want %v naming both documents", err, ErrDebugTarget)
	}
	var wire *jsonrpc2.Error
	if !errors.As(err, &wire) || wire.Code != jsonrpc2.InvalidParams {
		t.Errorf("err = %v, want an InvalidParams reply", err)
	}
}

// Closing the target's or the object's document ends a session even when the
// file is on disk, so the workspace goes on holding the document it declares.
func TestDebugEndsWhenTargetOrObjectDocumentClosedOnDisk(t *testing.T) {
	ctx := context.Background()
	split := strings.Index(debugMachine, "package MachineViews")
	machines, views := debugMachine[:split], debugMachine[split:]
	robots := "package Robots { part def Rover { exhibit state ops : Machines::Ops; } }\n"
	for _, tc := range []struct{ closed, decl string }{
		{"machines.sysml", "Machines::Ops"},
		{"robots.sysml", "Robots::Rover"},
	} {
		t.Run(tc.closed+" closed", func(t *testing.T) {
			dir := t.TempDir()
			for name, src := range map[string]string{"views.sysml": views, "machines.sysml": machines, "robots.sysml": robots} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			s, viewsURI, rec := debugServer(t, filepath.Join(dir, "views.sysml"), views)
			for name, src := range map[string]string{"machines.sysml": machines, "robots.sysml": robots} {
				if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
					TextDocument: protocol.TextDocumentItem{URI: uri.File(filepath.Join(dir, name)), LanguageID: "sysml", Version: 1, Text: src},
				}); err != nil {
					t.Fatalf("DidOpen %s: %v", name, err)
				}
			}
			snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: viewsURI},
				View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Robots::Rover",
			})
			closed := filepath.Join(dir, tc.closed)
			if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(closed)}}); err != nil {
				t.Fatalf("DidClose: %v", err)
			}
			if s.ws.Document(closed) == nil {
				t.Fatalf("closing dropped %s, which is on disk", tc.closed)
			}
			changed := rec.debugChanged()
			if len(changed) != 1 {
				t.Fatalf("debugChanged = %d notifications, want the one ending the session", len(changed))
			}
			last := changed[0]
			if last.Session != snap.Session || last.State != debugEnded || !strings.Contains(last.Reason, tc.closed+" was closed") {
				t.Fatalf("debugChanged after closing %s's document = %s", tc.decl, describe(last))
			}
			if _, err := s.DebugStep(&debugSessionParams{Session: snap.Session}); !errors.Is(err, ErrDebugSession) {
				t.Errorf("step after the close: err = %v, want %v", err, ErrDebugSession)
			}
		})
	}
}

// debugReads is a machine and an action whose run reads beyond what they
// inherit: a performer's supertype and feature types, an invoked action, a
// signal's schema and a value named from another package.
const debugReads = `package Consts {
	private import ScalarValues::*;
	attribute threshold : Integer = 2;
	attribute boost : Integer = 7;
}
package Deps {
	private import ScalarValues::*;
	attribute def Speed { attribute level : Integer; }
	item def Cargo { attribute mass : Real = 1.0; }
	part def Base { attribute limit : Integer = 3; }
	action def Brake { action grip; }
	action def Drive {
		action brake : Brake;
		first start;
		then brake;
		then done;
	}
	state def Ops {
		entry; then idle;
		state idle;
		transition first idle accept s : Speed if s.level > Consts::threshold then run;
		state run;
	}
	package Bots {
		part def Robot :> Base {
			attribute load : Cargo;
			perform action drive : Drive;
			exhibit state ops : Ops;
		}
	}
}
package DepViews {
	private import StandardViewDefinitions::*;
	view opsView : StateTransitionView { expose Deps::Ops; }
	view driveView : ActionFlowView { expose Deps::Drive; }
}
`

// A session ends when any declaration its run reads is edited: the performer's
// supertype or a feature's type, an invoked action, a signal's schema, a value a
// guard names, or one a send's argument named; and only when one is.
func TestDebugSessionEndsWhenWhatItReadsChanges(t *testing.T) {
	ctx := context.Background()
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
	cases := []struct {
		name, view, target, object, old, new, reason string
	}{
		{"performer supertype", "DepViews::opsView", "Deps::Ops", "Deps::Bots::Robot",
			"limit : Integer = 3", "limit : Integer = 4", "Deps::Base was edited"},
		{"performer feature type", "DepViews::opsView", "Deps::Ops", "Deps::Bots::Robot",
			"mass : Real = 1.0", "mass : Real = 2.0", "Deps::Cargo was edited"},
		{"invoked action", "DepViews::driveView", "Deps::Drive", "",
			"action grip;", "action grip; action release;", "Deps::Brake was edited"},
		{"signal schema", "DepViews::opsView", "Deps::Ops", "",
			"attribute level : Integer;", "attribute level : Real;", "Deps::Speed was edited"},
		{"value a guard names", "DepViews::opsView", "Deps::Ops", "",
			"threshold : Integer = 2", "threshold : Integer = 5", "Consts::threshold was edited"},
		{"value a guard names is removed", "DepViews::opsView", "Deps::Ops", "",
			"threshold : Integer = 2", "limit : Integer = 2", "Consts::threshold is no longer declared"},
		{"performer supertype resolves elsewhere", "DepViews::opsView", "Deps::Ops", "Deps::Bots::Robot",
			"package Bots {", "package Bots {\n\t\tpart def Base;", "Deps::Ops now reads Deps::Bots::Base"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, docURI, rec := debugServer(t, "/w/d.sysml", debugReads)
			snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: tc.view, Target: tc.target, Object: tc.object,
			})
			if strings.Count(debugReads, tc.old) != 1 {
				t.Fatalf("fixture writes %q %d times", tc.old, strings.Count(debugReads, tc.old))
			}
			s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: strings.Replace(debugReads, tc.old, tc.new, 1)}}, 2)
			if reason := lastEnd(t, rec, snap.Session).Reason; !strings.Contains(reason, tc.reason) {
				t.Errorf("reason = %q, want it to name %q", reason, tc.reason)
			}
		})
	}

	t.Run("signal a bare trigger names", func(t *testing.T) {
		s, docURI, rec := debugServer(t, "/w/m.sysml", debugMachine)
		snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "MachineViews::opsView", Target: "Machines::Ops",
		})
		edited := strings.Replace(debugMachine, "attribute def Halt;", "attribute def Halt { attribute why : Integer; }", 1)
		s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: edited}}, 2)
		if reason := lastEnd(t, rec, snap.Session).Reason; !strings.Contains(reason, "Machines::Halt was edited") {
			t.Errorf("reason = %q, want it to name Machines::Halt", reason)
		}
	})

	t.Run("value a send named", func(t *testing.T) {
		s, docURI, rec := debugServer(t, "/w/d.sysml", debugReads)
		snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "DepViews::opsView", Target: "Deps::Ops",
		})
		edited := strings.Replace(debugReads, "boost : Integer = 7", "boost : Integer = 8", 1)
		s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: edited}}, 2)
		if changed := rec.debugChanged(); len(changed) != 1 || changed[0].State == debugEnded {
			t.Fatalf("debugChanged after editing a value the run has not read = %v", changed)
		}
		sent := mustDebug(t, s, MethodDebugSend, &debugSendParams{
			Session: snap.Session, Signal: "Speed", Args: map[string]string{"level": "Consts::boost"},
		})
		if sent.State != debugEnded || !strings.Contains(sent.Reason, "Consts::boost was edited") {
			t.Fatalf("send naming an edited value = %s, want ended for Consts::boost", describe(sent))
		}
		if _, err := s.DebugStep(&debugSessionParams{Session: snap.Session}); !errors.Is(err, ErrDebugSession) {
			t.Errorf("step after the end: %v, want %v", err, ErrDebugSession)
		}

		s, docURI, rec = debugServer(t, "/w/d.sysml", debugReads)
		snap = mustDebug(t, s, MethodDebugStart, &debugStartParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "DepViews::opsView", Target: "Deps::Ops",
		})
		sent = mustDebug(t, s, MethodDebugSend, &debugSendParams{
			Session: snap.Session, Signal: "Speed", Args: map[string]string{"level": "Consts::boost"},
		})
		if sent.State == debugEnded {
			t.Fatalf("send naming an unchanged value = %s", describe(sent))
		}
		s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: edited}}, 2)
		if reason := lastEnd(t, rec, snap.Session).Reason; !strings.Contains(reason, "Consts::boost was edited") {
			t.Errorf("reason = %q, want it to name Consts::boost", reason)
		}
	})

	// An edit to a declaration the run never reads leaves the session running,
	// as does one to the notes and comments after declarations it does read.
	unread := []struct{ name, old, new string }{
		{"unrelated edit", "boost : Integer = 7", "boost : Integer = 8"},
		{"note after a read declaration", "attribute threshold : Integer = 2;", "attribute threshold : Integer = 2; // the limit"},
		{"comment after the target", "state run;\n\t}", "state run;\n\t} /* the machine */"},
		{"note after the view", "expose Deps::Ops; }", "expose Deps::Ops; } // drawn"},
		{"comment after the view", "expose Deps::Ops; }", "expose Deps::Ops; } /* drawn */"},
	}
	for _, tc := range unread {
		t.Run(tc.name, func(t *testing.T) {
			s, docURI, rec := debugServer(t, "/w/d.sysml", debugReads)
			snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "DepViews::opsView", Target: "Deps::Ops", Object: "Deps::Bots::Robot",
			})
			s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{{Text: strings.Replace(debugReads, tc.old, tc.new, 1)}}, 2)
			changed := rec.debugChanged()
			if len(changed) != 1 || changed[0].Session != snap.Session || changed[0].State == debugEnded {
				t.Fatalf("debugChanged after the edit = %v", changed)
			}
			stepped := mustDebug(t, s, MethodDebugStep, &debugSessionParams{Session: snap.Session})
			if stepped.State == debugEnded {
				t.Fatalf("step after the edit = %s", describe(stepped))
			}
		})
	}
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
