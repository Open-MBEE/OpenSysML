package sessiontools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	engineset "github.com/Open-MBEE/OpenSysML/internal/exec/engines"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/all" // the notation and graph writers the tools use
)

// connect serves the session tools over root to an in-memory client.
func connect(t *testing.T, root string) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	srv := sdk.NewServer(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	m := NewManager(root, func() (*repl.Session, error) {
		s := repl.NewSession()
		return s, s.SetEngines(engineset.Default())
	})
	Register(srv, m)
	t.Cleanup(m.CloseAll)
	serverT, clientT := sdk.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverT) }()
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// call makes one tool call and decodes its structured content into out; a
// failed call is returned for the caller to judge.
func call(t *testing.T, cs *sdk.ClientSession, name string, args any, out any) *sdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if out != nil && !res.IsError {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("%s: marshal structured content: %v", name, err)
		}
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("%s: structured content %s: %v", name, raw, err)
		}
	}
	return res
}

// must is call for a tool that has to succeed.
func must(t *testing.T, cs *sdk.ClientSession, name string, args any, out any) {
	t.Helper()
	if res := call(t, cs, name, args, out); res.IsError {
		t.Fatalf("%s(%v) failed: %s", name, args, errText(res))
	}
}

// errText is what a failed call said.
func errText(res *sdk.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func args(kv ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

const actionModel = `package Debug {
	action tally {
		attribute total = 0;
		first start;
		action accumulate { assign total := total + 5; }
		done;
		succession first start then accumulate;
		succession first accumulate then done;
	}
}
`

const machineModel = `package Lamps {
	attribute def Go;
	state def Lamp {
		entry; then off;
		state off;
		transition first off accept Go then on;
		state on;
	}
	part def Bulb { exhibit state light : Lamp; }
	part bulb : Bulb;
}
`

const solverModel = `package Check {
	private import ScalarValues::Integer;
	constraint def Satisfiable {
		in i : Integer;
		assert constraint { i > 3 and i < 8 }
	}
	constraint def Contradictory {
		in i : Integer;
		assert constraint { i > 8 and i < 3 }
	}
}
`

const calcModel = `package Calcs {
	private import ScalarValues::Real;
	calc def Double { in x : Real; return : Real = x * 2.0; }
}
`

func TestLoadSubmitAndEvaluate(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "veh.sysml"), []byte("package Veh { part def Car { attribute mass : ScalarValues::Real = 1200.0; } part car : Car; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cs := connect(t, root)

	var loaded LoadResult
	must(t, cs, "session_load", args("paths", []string{"veh.sysml"}), &loaded)
	if !slices.Equal(loaded.Files, []string{filepath.Join(root, "veh.sysml")}) {
		t.Errorf("files = %v", loaded.Files)
	}
	if loaded.Errors || len(loaded.Diagnostics) != 0 {
		t.Errorf("load reported errors: %+v", loaded)
	}
	if !strings.Contains(strings.Join(loaded.Declared, "\n"), "Veh") {
		t.Errorf("declared = %v", loaded.Declared)
	}

	var submitted SubmitResult
	must(t, cs, "session_submit", args("text", "package Extra { part def Trailer :> Veh::Car; }"), &submitted)
	if !submitted.Accepted || !slices.Contains(submitted.Declared, "Extra") {
		t.Errorf("submit = %+v", submitted)
	}

	must(t, cs, "session_submit", args("text", "package Broken { part def P :> Missing; }"), &submitted)
	if submitted.Accepted || len(submitted.Diagnostics) == 0 || submitted.Diagnostics[0].Severity != "error" || submitted.Diagnostics[0].Line != 1 {
		t.Errorf("a submission with an unresolved name was reported as %+v", submitted)
	}

	var ev EvalResult
	must(t, cs, "session_eval", args("expression", "Veh::car.mass * 2"), &ev)
	if ev.Text != "2400.0" {
		t.Errorf("eval text = %q (output %q)", ev.Text, ev.Output)
	}
	value, _ := ev.Value.(map[string]any)
	if value == nil || len(value) == 0 {
		t.Errorf("eval value = %#v, want the API's typed value", ev.Value)
	}
	if res := call(t, cs, "session_eval", args("expression", "nosuch + 1"), nil); !res.IsError {
		t.Error("evaluating an unknown name succeeded")
	}

	var printed Notation
	must(t, cs, "session_print", args("name", "Extra"), &printed)
	if !strings.Contains(printed.Text, "Trailer") {
		t.Errorf("printed %q", printed.Text)
	}

	var cleared ClearResult
	must(t, cs, "session_clear", args(), &cleared)
	var cmd CommandResult
	must(t, cs, "repl_command", args("line", "%list"), &cmd)
	if cmd.Output != "(empty session)" {
		t.Errorf("after clear, %%list printed %q", cmd.Output)
	}
}

func TestSaveWritesOnlyWhenApplied(t *testing.T) {
	root := t.TempDir()
	cs := connect(t, root)
	must(t, cs, "session_submit", args("text", "package Saved { part def P; }"), nil)

	var saved SaveResult
	must(t, cs, "session_save", args("path", "out/model.sysml"), &saved)
	if saved.Applied || !strings.Contains(saved.Text, "Saved") {
		t.Errorf("preview = %+v", saved)
	}
	if _, err := os.Stat(filepath.Join(root, "out", "model.sysml")); !os.IsNotExist(err) {
		t.Fatalf("a save without apply touched the disk: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "out"), 0o750); err != nil {
		t.Fatal(err)
	}
	must(t, cs, "session_save", args("path", "out/model.sysml", "apply", true), &saved)
	if !saved.Applied || saved.Path != filepath.Join(root, "out", "model.sysml") {
		t.Errorf("save = %+v", saved)
	}
	data, err := os.ReadFile(saved.Path)
	if err != nil || !strings.Contains(string(data), "part def P") {
		t.Errorf("written file = %q, %v", data, err)
	}
}

func TestDebuggerStepsAnActionToCompletion(t *testing.T) {
	cs := connect(t, t.TempDir())
	must(t, cs, "session_submit", args("text", actionModel), nil)

	var st DebugState
	must(t, cs, "debug_start", args("kind", "action", "name", "Debug::tally"), &st)
	if st.Kind != "action" || st.Ended || len(st.Tokens) != 1 || st.Tokens[0].Node != "start" {
		t.Fatalf("started = %+v", st)
	}
	must(t, cs, "debug_breakpoints", args("set", []string{"accumulate"}), &st)
	if !slices.Equal(st.Breakpoints, []string{"accumulate"}) {
		t.Errorf("breakpoints = %v", st.Breakpoints)
	}
	if res := call(t, cs, "debug_breakpoints", args("set", []string{"nowhere"}), nil); !res.IsError || !strings.Contains(errText(res), "nodes:") {
		t.Errorf("an unknown node was accepted: %+v", res)
	}
	must(t, cs, "debug_continue", args(), &st)
	if st.PausedAt != "accumulate" || st.Ended {
		t.Fatalf("continue did not stop at the breakpoint: %+v", st)
	}
	must(t, cs, "debug_current", args(), &st)
	if len(st.Tokens) != 1 || !strings.Contains(st.Text, "Active tokens") {
		t.Errorf("current = %+v", st)
	}
	must(t, cs, "debug_step", args(), &st)
	must(t, cs, "debug_continue", args(), &st)
	if !st.Ended || st.State != "Completed" {
		t.Fatalf("after continue: %+v", st)
	}
	if !slices.Contains(st.Results, NamedValue{Name: "total", Value: "5"}) {
		t.Errorf("results = %v", st.Results)
	}
	var out Output
	must(t, cs, "debug_stop", args(), &out)
	if res := call(t, cs, "debug_step", args(), nil); !res.IsError {
		t.Error("stepping after stop succeeded")
	}
}

func TestDebuggerDrivesAMachineWithSignals(t *testing.T) {
	cs := connect(t, t.TempDir())
	must(t, cs, "session_submit", args("text", machineModel), nil)
	var cmd CommandResult
	must(t, cs, "repl_command", args("line", "%instantiate Lamps::bulb"), &cmd)

	var held Instances
	must(t, cs, "instances", args(), &held)
	if len(held.Instances) != 1 || held.Instances[0].Type != "Lamps::bulb" || held.Instances[0].ID != 1 {
		t.Fatalf("instances = %+v", held)
	}

	var st DebugState
	must(t, cs, "debug_start", args("kind", "state", "name", "bulb"), &st)
	if st.Kind != "state" || st.CurrentState != "off" {
		t.Fatalf("started = %+v", st)
	}
	var sent Output
	must(t, cs, "send", args("signal", "Lamps::Go"), &sent)
	if !strings.Contains(sent.Text, "Sent") {
		t.Errorf("send printed %q", sent.Text)
	}
	must(t, cs, "debug_current", args(), &st)
	if len(st.SignalsInFlight) != 1 {
		t.Errorf("signals in flight = %v", st.SignalsInFlight)
	}
	must(t, cs, "advance", args("time", 0), &st)
	if st.CurrentState != "on" {
		t.Fatalf("after the signal: %+v", st)
	}

	var feats Features
	must(t, cs, "features", args("object", "bulb"), &feats)
	if !strings.Contains(feats.Text, "Instance: Lamps::bulb (ID: 1)") || feats.Graph == nil {
		t.Errorf("features = %+v", feats)
	}
	if res := call(t, cs, "features", args("object", "nobody"), nil); !res.IsError {
		t.Error("an unknown object was listed")
	}
}

func TestSolverChecksAndExplains(t *testing.T) {
	if _, err := solve.Discover(); err != nil {
		t.Skipf("no SMT solver installed: %v", err)
	}
	cs := connect(t, t.TempDir())
	must(t, cs, "session_submit", args("text", solverModel), nil)

	var res SolveResult
	must(t, cs, "check", args("name", "Check::Satisfiable"), &res)
	if len(res.Reports) != 1 || res.Reports[0].Status != "sat" || !res.Reports[0].Satisfiable || res.Reports[0].Solver == "" {
		t.Fatalf("check = %+v", res)
	}
	must(t, cs, "explain", args("name", "Check::Contradictory"), &res)
	if len(res.Reports) != 1 || res.Reports[0].Status != "unsat" || !strings.Contains(res.Text, "conflict") {
		t.Fatalf("explain = %+v", res)
	}
}

func TestSweepTablesEveryRow(t *testing.T) {
	cs := connect(t, t.TempDir())
	must(t, cs, "session_submit", args("text", calcModel), nil)

	var v Verdict
	must(t, cs, "sweep", args("name", "Calcs::Double", "ranges", []string{"x=1..3:1"}), &v)
	if v.Status != "holds" || len(v.Rows) != 3 {
		t.Fatalf("sweep = %+v", v)
	}
	if !slices.Contains(v.Rows[2].Outputs, NamedValue{Name: "result", Value: "6.0"}) {
		t.Errorf("third row = %+v", v.Rows[2])
	}
	var run Verdict
	must(t, cs, "session_run", args("kind", "calc", "name", "Calcs::Double", "arguments", []string{"4.0"}), &run)
	if run.Status != "holds" || !slices.Contains(run.Values, NamedValue{Name: "result", Value: "8.0"}) {
		t.Errorf("run = %+v", run)
	}
	if res := call(t, cs, "session_run", args("kind", "dance", "name", "Calcs::Double"), nil); !res.IsError {
		t.Error("an unknown kind ran")
	}
}

func TestReplCommandReachesCommandsWithoutATypedTool(t *testing.T) {
	cs := connect(t, t.TempDir())
	var cmd CommandResult
	must(t, cs, "repl_command", args("line", "%verbosity"), &cmd)
	if cmd.Output != "verbosity: normal" {
		t.Errorf("%%verbosity printed %q", cmd.Output)
	}
	must(t, cs, "repl_command", args("line", "%help"), &cmd)
	if !strings.Contains(cmd.Output, "%instantiate") {
		t.Errorf("%%help printed %q", cmd.Output)
	}
	if res := call(t, cs, "repl_command", args("line", "part def P;"), nil); !res.IsError {
		t.Error("a declaration was accepted as a command")
	}
	if res := call(t, cs, "repl_command", args("line", "%instantiate Nowhere::x"), nil); !res.IsError {
		t.Error("instantiating an unknown name succeeded")
	}
}

func TestAnUnknownSessionIsRefused(t *testing.T) {
	cs := connect(t, t.TempDir())
	res := call(t, cs, "session_eval", args("session", "nope", "expression", "1"), nil)
	if !res.IsError || !strings.Contains(errText(res), "no such session") {
		t.Errorf("unknown session: %+v", res)
	}
}

func TestSessionsAreIndependent(t *testing.T) {
	cs := connect(t, t.TempDir())
	must(t, cs, "session_open", args("session", "other"), nil)
	must(t, cs, "session_submit", args("text", "package A { attribute x = 1; }"), nil)
	must(t, cs, "session_submit", args("session", "other", "text", "package A { attribute x = 2; }"), nil)
	var ev EvalResult
	must(t, cs, "session_eval", args("expression", "A::x"), &ev)
	other := EvalResult{}
	must(t, cs, "session_eval", args("session", "other", "expression", "A::x"), &other)
	if ev.Text != "1" || other.Text != "2" {
		t.Errorf("default = %q, other = %q", ev.Text, other.Text)
	}
	must(t, cs, "session_close", args("session", "other"), nil)
	var list SessionList
	must(t, cs, "session_list", args(), &list)
	if !slices.Equal(list.Sessions, []string{"default"}) {
		t.Errorf("sessions = %v", list.Sessions)
	}
}

func TestConcurrentCallsOnOneSessionAreSerialized(t *testing.T) {
	cs := connect(t, t.TempDir())
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan string, 2*n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := string(rune('A' + i))
			res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "session_submit", Arguments: args("text", "package "+name+" { attribute v = "+string(rune('0'+i))+"; }")})
			if err != nil || res.IsError {
				errs <- "submit " + name + " failed"
				return
			}
			res, err = cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "session_eval", Arguments: args("expression", name+"::v")})
			if err != nil || res.IsError {
				errs <- "eval " + name + " failed"
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	var cmd CommandResult
	must(t, cs, "repl_command", args("line", "%list"), &cmd)
	for i := range n {
		if !strings.Contains(cmd.Output, "package "+string(rune('A'+i))) {
			t.Errorf("%%list lacks %c: %q", 'A'+i, cmd.Output)
		}
	}
}
