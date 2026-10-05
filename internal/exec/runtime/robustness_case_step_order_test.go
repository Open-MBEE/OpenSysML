package runtime

import (
	"context"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestRuntimeRobustnessCaseStepOrder(t *testing.T) {
	t.Run("explore_budget_is_incomplete", func(t *testing.T) {
		source := `package test {
			analysis def ManySteps {
				return : Integer;
				attribute x : Integer := 0;
				action s1 { assign x := x + 1; }
				action s2 { assign x := x + 1; }
				action s3 { assign x := x + 1; }
				action s4 { assign x := x + 1; }
				action s5 { assign x := x + 1; }
				action s6 { assign x := x + 1; }
				action s7 { assign x := x + 1; }
				x
			}
		}`
		m := parseLibraryModel(t, source)
		sym := namedOrFoundSymbol(t, m.idx, "test::ManySteps", m.idx.DocumentRoot(m.path), ast.DefAnalysisCase, ast.UsageAnalysisCase)
		policy, err := ExplorePolicy(DefaultExploreBudget)
		if err != nil {
			t.Fatal(err)
		}
		exploration, err := Explore(context.Background(), policy, m.fresh, func(ctx *Context) (Outcome, error) {
			result, err := ctx.RunAnalysis(sym, AnalysisArgs{}, m.idx.DocumentRoot(m.path), nil)
			if err != nil {
				return Outcome{}, err
			}
			outputs := make(map[string]Value, len(result.Outputs))
			for _, output := range result.Outputs {
				outputs[output.Name] = output.Value
			}
			return Outcome{Outputs: outputs}, nil
		})
		if err != nil {
			t.Fatalf("explore many-step case: %v", err)
		}
		if exploration.Complete() || !slices.Contains(exploration.BudgetsHit, "runs") {
			t.Fatalf("exploration %s after %d runs, want incomplete run-budget status", exploration.Status(), exploration.Runs)
		}
	})
}
