package smt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const solverRequiredEnv = "OPENSYSML_REQUIRE_SMT"

// requireSolver returns the discovered solver, skipping without one unless
// the environment declares one mandatory.
func requireSolver(t *testing.T) *solve.Solver {
	t.Helper()
	solver, err := solve.Discover()
	if err == nil {
		return solver
	}
	if !errors.Is(err, solve.ErrNoSolver) {
		t.Fatalf("discover a solver: %v", err)
	}
	if os.Getenv(solverRequiredEnv) != "" {
		t.Fatalf("%s=%s but %v", solverRequiredEnv, os.Getenv(solverRequiredEnv), err)
	}
	t.Skipf("no SMT solver installed: %v", err)
	return nil
}

// loweredAction indexes src and starts the action fqn declares, as the engine
// does: the flow it performs and what the performance holds at its start.
func loweredAction(t *testing.T, src, fqn string) (*runtime.Context, *symbols.Symbol, *lower.ActionGraph, runtime.Held) {
	t.Helper()
	return loweredDocument(t, "encode_test.sysml", src, fqn)
}

// loweredConformanceAction starts the named action of a conformance case with
// a context to encode it in.
func loweredConformanceAction(t *testing.T, file, fqn string) (*runtime.Context, *symbols.Symbol, *lower.ActionGraph, runtime.Held) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(conformanceDir, file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return loweredDocument(t, file, string(src), fqn)
}

func loweredDocument(t *testing.T, path, src, fqn string) (*runtime.Context, *symbols.Symbol, *lower.ActionGraph, runtime.Held) {
	t.Helper()
	ctx, idx := fixture(t, path, src)
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", fqn, len(matches))
	}
	graph, held := started(t, ctx, matches[0])
	return ctx, matches[0], graph, held
}

// started begins one performance of action and reports the flow it performs
// and what it holds at its start, which is what Encode is given.
func started(t *testing.T, ctx *runtime.Context, action *symbols.Symbol) (*lower.ActionGraph, runtime.Held) {
	t.Helper()
	exec, err := ctx.CreateActionExecutor(action)
	if err != nil {
		t.Fatalf("start %s: %v", action.Name, err)
	}
	defer exec.Release()
	return exec.Graph(), exec.Held()
}

// modelText renders a satisfying assignment, one variable per line, sorted.
func modelText(result *solve.Result) string {
	lines := make([]string, 0, len(result.Model))
	for _, a := range result.Model {
		lines = append(lines, a.Var.Name+" = "+a.Value)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// assigned is the value the model gives the named variable.
func assigned(t *testing.T, result *solve.Result, name string) string {
	t.Helper()
	for _, a := range result.Model {
		if a.Var.Name == name {
			return a.Value
		}
	}
	t.Fatalf("no assignment to %s in:\n%s", name, modelText(result))
	return ""
}

// TestEncodeForkJoinCompletes: the fork/join case reaches its final node
// within six moves, with both flags true, and cannot do so within five.
func TestEncodeForkJoinCompletes(t *testing.T) {
	solver := requireSolver(t)
	src := `package test {
	private import ScalarValues::*;
	action clash {
		attribute x : Integer = 0;
		attribute leftRan : Boolean = false;
		attribute rightRan : Boolean = false;
		first start;
		fork split;
		action left { assign x := 1; assign leftRan := true; }
		action right { assign x := 2; assign rightRan := true; }
		join sync;
		done;
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}
}`
	for _, k := range []int{6, 7} {
		t.Run(fmt.Sprintf("k=%d", k), func(t *testing.T) {
			ctx, action, graph, held := loweredAction(t, src, "test::clash")
			enc, err := Encode(ctx, action, graph, held, nil, k, DefaultUnroll)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			q := *enc.Query
			q.Assertions = append(q.Assertions, solve.Assertion{Term: enc.Completed[k]})
			result, err := solver.Solve(context.Background(), &q)
			if err != nil {
				t.Fatalf("solve: %v\n%s", err, solve.Script(&q))
			}
			if result.Status != solve.StatusSat {
				t.Fatalf("status %v, want sat\n%s", result.Status, solve.Script(&q))
			}
			t.Logf("model:\n%s", modelText(result))
			last := enc.States[k]
			for _, base := range enc.Features {
				v := last.value(base)
				if strings.HasSuffix(base.Name, "Ran") && assigned(t, result, v.Name) != "true" {
					t.Errorf("%s = %s at move %d, want true", v.Name, assigned(t, result, v.Name), k)
				}
			}
		})
	}
	ctx, action, graph, held := loweredAction(t, src, "test::clash")
	enc, err := Encode(ctx, action, graph, held, nil, 5, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	q := *enc.Query
	q.Assertions = append(q.Assertions, solve.Assertion{Term: enc.Completed[5]})
	result, err := solver.Solve(context.Background(), &q)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != solve.StatusUnsat {
		t.Fatalf("completes within 5 moves: %v\n%s", result.Status, modelText(result))
	}
}

// status solves enc's query with the completion at k and extra asserted.
func status(t *testing.T, solver *solve.Solver, enc *Encoding, k int, extra *solve.Term) solve.Status {
	t.Helper()
	q := *enc.Query
	q.Assertions = append(q.Assertions, solve.Assertion{Term: enc.Completed[k]}, solve.Assertion{Term: extra})
	result, err := solver.Solve(context.Background(), &q)
	if err != nil {
		t.Fatalf("solve: %v\n%s", err, solve.Script(&q))
	}
	return result.Status
}

// TestEncodePinsAndObjectFlows: the pin and object-flow conformance cases complete with the
// interpreter's values, and a node reading a pin not yet delivered fails as the interpreter does.
func TestEncodePinsAndObjectFlows(t *testing.T) {
	solver := requireSolver(t)
	const k = 8
	cases := []struct {
		file, fqn string
		fails     bool
		values    map[string]int64
	}{
		{"action_flow_between_same_named_pins.sysml", "test::outer", false,
			map[string]int64{"test::outer::result": 21, "test::outer::p::v": 21, "test::outer::q::v": 21, "test::outer::q::w": 42}},
		{"action_node_pins_isolated.sysml", "test::outer", false,
			map[string]int64{"test::outer::total": 7, "test::outer::p::v": 3, "test::outer::q::v": 4}},
		{"action_flow_named_from.sysml", "test::driveTrain", false,
			map[string]int64{"test::driveTrain::generateTorque::engineTorque": 21, "test::driveTrain::amplifyTorque::torqueIn": 21, "test::driveTrain::amplifyTorque::amplified": 42}},
		{"action_node_pin_read_before_performed.sysml", "test::outer", true, nil},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			ctx, action, graph, held := loweredConformanceAction(t, c.file, c.fqn)
			enc, err := Encode(ctx, action, graph, held, nil, k, DefaultUnroll)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			last := enc.States[k]
			failed := solve.VarTerm(last.Failed)
			if got := status(t, solver, enc, k, failed); (got == solve.StatusSat) != c.fails {
				t.Errorf("completes failed: %v, want fails=%v", got, c.fails)
			}
			if got := status(t, solver, enc, k, solve.Not(failed)); (got == solve.StatusSat) == c.fails {
				t.Errorf("completes unfailed: %v, want fails=%v", got, c.fails)
			}
			for name, want := range c.values {
				v := last.Values[name]
				if v == nil {
					t.Errorf("no feature %s among %v", name, names(enc.Features))
					continue
				}
				is := eq(solve.VarTerm(v), solve.IntTerm(want))
				if got := status(t, solver, enc, k, is); got != solve.StatusSat {
					t.Errorf("%s = %d on completion: %v, want sat", name, want, got)
				}
				if got := status(t, solver, enc, k, solve.And(solve.Not(failed), solve.Not(is))); got != solve.StatusUnsat {
					t.Errorf("%s != %d on completion: %v, want unsat", name, want, got)
				}
			}
		})
	}
}

// names lists the variables' names.
func names(vars []*solve.Var) []string {
	out := make([]string, len(vars))
	for i, v := range vars {
		out[i] = v.Name
	}
	return out
}

// TestEncodeInlineExpressionFeedsFlow: an inline expression node holds its
// value in the feature it writes before the object flow out of it delivers, as
// the interpreter orders them, so the pin the flow feeds reads the value. The
// notation declares no such node, so the graph is lowered from one built as the
// interpreter's own tests build it, over the scope of a parsed action.
func TestEncodeInlineExpressionFeedsFlow(t *testing.T) {
	solver := requireSolver(t)
	const k = 8
	ctx, idx := fixture(t, "inline_test.sysml", `package test {
	private import ScalarValues::*;
	action outer {
		attribute result : Integer;
		first start;
		action take { in v : Integer; }
		done;
		succession first start then compute;
		succession first compute then take;
		succession first take then done;
		flow from compute.result to take.v;
	}
}`)
	matches := idx.LookupQualified("test::outer")
	if len(matches) != 1 {
		t.Fatalf("test::outer matched %d symbols, want 1", len(matches))
	}
	parsed, ok := matches[0].Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("test::outer declared by %T, want a usage", matches[0].Decl)
	}
	compute := &ast.ActionExecutionNode{
		Name: "compute",
		Expression: &ast.OperatorExpr{
			Operator: ast.OpAdd,
			Operands: []ast.Node{&ast.LiteralInteger{Value: "3"}, &ast.LiteralInteger{Value: "4"}},
		},
	}
	decl := *parsed
	decl.Members = append(slices.Clone(parsed.Members), compute)
	action := &symbols.Symbol{Name: parsed.Ident.Name, Kind: symbols.SymbolActionUsage, Decl: &decl, Scope: matches[0].Scope}

	results, err := ctx.ExecuteAction(action)
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}
	if got := results["result"]; got.Kind != runtime.ValConst || got.Const.Int != 7 {
		t.Fatalf("interpreted result = %v, want 7", got)
	}

	graph, held := started(t, ctx, action)
	enc, err := Encode(ctx, action, graph, held, nil, k, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	last := enc.States[k]
	failed := solve.VarTerm(last.Failed)
	if got := status(t, solver, enc, k, solve.Not(failed)); got != solve.StatusSat {
		t.Fatalf("completes unfailed: %v, want sat", got)
	}
	for name, want := range map[string]int64{"test::outer::result": 7, "test::outer::take::v": 7} {
		v := last.Values[name]
		if v == nil {
			t.Errorf("no feature %s among %v", name, names(enc.Features))
			continue
		}
		is := eq(solve.VarTerm(v), solve.IntTerm(want))
		if got := status(t, solver, enc, k, solve.And(solve.Not(failed), is)); got != solve.StatusSat {
			t.Errorf("%s = %d on completion: %v, want sat", name, want, got)
		}
		if got := status(t, solver, enc, k, solve.And(solve.Not(failed), solve.Not(is))); got != solve.StatusUnsat {
			t.Errorf("%s != %d on completion: %v, want unsat", name, want, got)
		}
	}
}

// TestEncodeFlowKindsDiffer: a plain flow streams the write to its source pin
// into a consumer performing beside the producer, which fails when the consumer
// completed first and reads 7 otherwise; a succession flow orders the consumer
// after the producer and hands it the value, so no schedule fails.
func TestEncodeFlowKindsDiffer(t *testing.T) {
	solver := requireSolver(t)
	const k = 12
	const forked = `
		fork split;
		join sync;
		succession first start then split;
		succession first split then producer;
		succession first split then consumer;
		succession first producer then sync;
		succession first consumer then sync;
		succession first sync then done;
		flow producer.value to consumer.got;`
	const ordered = `
		succession first start then producer;
		succession first consumer then done;
		succession flow producer.value to consumer.got;`
	model := func(wiring string) string {
		return `package test {
	private import ScalarValues::*;
	action outer {
		attribute seen : Integer = -1;
		first start;
		action producer { out value : Integer = 5; assign value := 7; }
		action consumer { in got : Integer = 0; assign seen := got; }
		done;` + wiring + `
	}
}`
	}
	cases := []struct {
		name, wiring string
		fails        bool
	}{
		{"flow", forked, true},
		{"succession flow", ordered, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, action, graph, held := loweredDocument(t, "kinds_test.sysml", model(c.wiring), "test::outer")
			exploration := explore(t, runtime.DefaultExploreBudget, func() (*runtime.Context, error) {
				return runtime.NewContext(ctx.Model(), 10000), nil
			}, action)
			if !exploration.Complete() {
				t.Fatalf("exploration %s", exploration.Status())
			}
			failing, values := false, make(map[string]bool)
			for _, o := range exploration.Outcomes {
				if o.Outcome.Err != nil {
					failing = true
					continue
				}
				values[runtime.FormatValue(o.Outcome.Outputs["seen"])] = true
			}
			if failing != c.fails || len(values) != 1 || !values["7"] {
				t.Fatalf("the interpreter's outcomes: fails=%v, seen in %v; want fails=%v, seen = 7 only", failing, values, c.fails)
			}
			enc, err := Encode(ctx, action, graph, held, nil, k, DefaultUnroll)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			last := enc.States[k]
			failed := solve.VarTerm(last.Failed)
			seen := last.Values["test::outer::seen"]
			if seen == nil {
				t.Fatalf("no feature seen among %v", names(enc.Features))
			}
			is := eq(solve.VarTerm(seen), solve.IntTerm(7))
			if got := status(t, solver, enc, k, solve.And(solve.Not(failed), is)); got != solve.StatusSat {
				t.Errorf("seen = 7 on unfailed completion: %v, want sat", got)
			}
			if got := status(t, solver, enc, k, solve.And(solve.Not(failed), solve.Not(is))); got != solve.StatusUnsat {
				t.Errorf("seen != 7 on unfailed completion: %v, want unsat", got)
			}
			if got := status(t, solver, enc, k, failed); (got == solve.StatusSat) != c.fails {
				t.Errorf("completes failed: %v, want fails=%v", got, c.fails)
			}
		})
	}
}

func TestEncodeStreamsConditionalWrite(t *testing.T) {
	solver := requireSolver(t)
	const k = 10
	ctx, action, graph, held := loweredDocument(t, "conditional_stream_test.sysml", `package test {
	private import ScalarValues::*;
	action outer {
		in enabled : Boolean;
		attribute seen : Integer = -1;
		first start;
		action producer { out value : Integer = 5; if enabled { assign value := 7; } }
		action consumer { in got : Integer = 0; assign seen := got; }
		done;
		succession first start then producer;
		succession first producer then consumer;
		succession first consumer then done;
		flow producer.value to consumer.got;
	}
}`, "test::outer")
	enc, err := Encode(ctx, action, graph, held, nil, k, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	last := enc.States[k]
	failed := solve.VarTerm(last.Failed)
	seen, enabled := last.Values["test::outer::seen"], last.Values["test::outer::enabled"]
	if seen == nil || enabled == nil {
		t.Fatalf("no features seen and enabled among %v", names(enc.Features))
	}
	if got := status(t, solver, enc, k, failed); got != solve.StatusUnsat {
		t.Errorf("completes failed: %v, want unsat", got)
	}
	agree := eq(solve.VarTerm(seen), solve.Ite(solve.VarTerm(enabled), solve.IntTerm(7), solve.IntTerm(5)))
	if got := status(t, solver, enc, k, solve.Not(agree)); got != solve.StatusUnsat {
		t.Errorf("seen disagrees with the branch taken: %v, want unsat", got)
	}
}

// TestEncodeStreamsBackToOwnPin: a plain flow from one pin of a node to another
// reaches the performance under way, whatever order the pins are declared in and
// whether the value it carries is the pin's declared one or a write in the body,
// and streams on from there; flows leading it back are the interpreter's cycle error.
func TestEncodeStreamsBackToOwnPin(t *testing.T) {
	solver := requireSolver(t)
	const k = 8
	cases := []struct {
		name, node, flows string
		want              int64
		cycle             bool
	}{
		{"target declared first", `action n { in back : Integer; out value : Integer = 5; assign seen := back; }`, `flow n.value to n.back;`, 5, false},
		{"source declared first", `action n { out value : Integer = 5; in back : Integer = 0; assign seen := back; }`, `flow n.value to n.back;`, 5, false},
		{"written in the body", `action n { in back : Integer; out value : Integer = 5; assign value := 7; assign seen := back; }`, `flow n.value to n.back;`, 7, false},
		{"two hops", `action n { in far : Integer; in back : Integer; out value : Integer = 5; assign seen := far; }`, `flow n.value to n.back; flow n.back to n.far;`, 5, false},
		{"cycle", `action n { in back : Integer; out value : Integer = 5; assign seen := back; }`, `flow n.value to n.back; flow n.back to n.value;`, 5, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, action, graph, held := loweredDocument(t, "own_pin_test.sysml", `package test {
	private import ScalarValues::*;
	action outer {
		attribute seen : Integer = -1;
		first start;
		`+c.node+`
		done;
		succession first start then n;
		succession first n then done;
		`+c.flows+`
	}
}`, "test::outer")
			exploration := explore(t, runtime.DefaultExploreBudget, func() (*runtime.Context, error) {
				return runtime.NewContext(ctx.Model(), 10000), nil
			}, action)
			if !exploration.Complete() {
				t.Fatalf("exploration %s", exploration.Status())
			}
			for _, o := range exploration.Outcomes {
				if c.cycle {
					if !errors.Is(o.Outcome.Err, runtime.ErrStreamCycle) {
						t.Fatalf("the interpreter: err=%v, want %v", o.Outcome.Err, runtime.ErrStreamCycle)
					}
					continue
				}
				if o.Outcome.Err != nil || o.Outcome.Outputs["seen"].Const.Int != c.want {
					t.Fatalf("the interpreter: err=%v seen=%s, want %d", o.Outcome.Err, runtime.FormatValue(o.Outcome.Outputs["seen"]), c.want)
				}
			}
			enc, err := Encode(ctx, action, graph, held, nil, k, DefaultUnroll)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			last := enc.States[k]
			failed := solve.VarTerm(last.Failed)
			if c.cycle {
				if got := status(t, solver, enc, k, solve.Not(failed)); got != solve.StatusUnsat {
					t.Errorf("unfailed: %v, want unsat", got)
				}
				return
			}
			seen := last.Values["test::outer::seen"]
			if seen == nil {
				t.Fatalf("no feature seen among %v", names(enc.Features))
			}
			is := eq(solve.VarTerm(seen), solve.IntTerm(c.want))
			if got := status(t, solver, enc, k, solve.And(solve.Not(failed), is)); got != solve.StatusSat {
				t.Errorf("seen = %d on unfailed completion: %v, want sat", c.want, got)
			}
			if got := status(t, solver, enc, k, solve.Or(failed, solve.Not(is))); got != solve.StatusUnsat {
				t.Errorf("fails or seen != %d: %v, want unsat", c.want, got)
			}
		})
	}
}
