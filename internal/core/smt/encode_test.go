package smt

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	ctx, idx := fixture(t, "encode_test.sysml", src)
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
