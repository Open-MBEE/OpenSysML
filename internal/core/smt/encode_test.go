package smt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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

// loweredAction indexes src and lowers the action fqn declares.
func loweredAction(t *testing.T, src, fqn string) (*runtime.Context, *symbols.Symbol, *lower.ActionGraph) {
	t.Helper()
	return loweredDocument(t, "encode_test.sysml", src, fqn)
}

// loweredConformanceAction lowers the named action of a conformance case with
// a context to encode it in.
func loweredConformanceAction(t *testing.T, file, fqn string) (*runtime.Context, *symbols.Symbol, *lower.ActionGraph) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(conformanceDir, file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return loweredDocument(t, file, string(src), fqn)
}

func loweredDocument(t *testing.T, path, src, fqn string) (*runtime.Context, *symbols.Symbol, *lower.ActionGraph) {
	t.Helper()
	ctx, idx := fixture(t, path, src)
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", fqn, len(matches))
	}
	graph, err := lower.ToActionGraph(matches[0].Decl, matches[0].Scope)
	if err != nil {
		t.Fatalf("lower %s: %v", fqn, err)
	}
	lower.StartFlow(graph)
	return ctx, matches[0], graph
}

// model renders a satisfying assignment, one variable per line, sorted.
func model(result *solve.Result) string {
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
	t.Fatalf("no assignment to %s in:\n%s", name, model(result))
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
			ctx, action, graph := loweredAction(t, src, "test::clash")
			enc, err := Encode(ctx, action, graph, k, DefaultUnroll)
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
			t.Logf("model:\n%s", model(result))
			last := enc.States[k]
			for _, base := range enc.Features {
				v := last.value(base)
				if strings.HasSuffix(base.Name, "Ran") && assigned(t, result, v.Name) != "true" {
					t.Errorf("%s = %s at move %d, want true", v.Name, assigned(t, result, v.Name), k)
				}
			}
		})
	}
	ctx, action, graph := loweredAction(t, src, "test::clash")
	enc, err := Encode(ctx, action, graph, 5, DefaultUnroll)
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
		t.Fatalf("completes within 5 moves: %v\n%s", result.Status, model(result))
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

// TestEncodePinsAndObjectFlows: the conformance cases over pins and object
// flows complete with exactly the values the interpreter's outcomes record,
// each node's pins being features of its own; a node reading a pin of a node
// not yet performed fails, as the interpreter does.
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
			ctx, action, graph := loweredConformanceAction(t, c.file, c.fqn)
			enc, err := Encode(ctx, action, graph, k, DefaultUnroll)
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
