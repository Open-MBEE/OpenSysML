package runtime

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestAnalysisCaseStepOrderOutcomes(t *testing.T) {
	m := caseStepOrderModel(t, "analysis_explore_step_order")
	sym := namedOrFoundSymbol(t, m.idx, "test::An", m.idx.DocumentRoot(m.path), ast.DefAnalysisCase, ast.UsageAnalysisCase)
	exploration := exploreCaseAnalysis(t, m, sym)
	if !exploration.Complete() || exploration.Runs != 2 {
		t.Fatalf("exploration %s after %d runs, want two complete runs", exploration.Status(), exploration.Runs)
	}
	if got := caseResultValues(t, exploration, "result"); !slices.Equal(got, []string{"12", "30"}) {
		t.Fatalf("analysis outcomes are %v, want [12 30]", got)
	}
	for _, outcome := range exploration.Outcomes {
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, ReplayPolicy(outcome.Witness))
		result, err := ctx.RunAnalysis(sym, AnalysisArgs{}, m.idx.DocumentRoot(m.path), nil)
		if err == nil {
			err = ctx.Unfollowed()
		}
		if err != nil {
			t.Fatalf("replay of %s: %v", FormatChoices(outcome.Witness), err)
		}
		if got := analysisResultValue(t, ctx, result, "result"); got != FormatValue(outcome.Outcome.Outputs["result"]) {
			t.Errorf("replay of %s produced %s, want %s",
				FormatChoices(outcome.Witness), got, FormatValue(outcome.Outcome.Outputs["result"]))
		}
	}

	for _, schedule := range []struct {
		policy string
		want   string
	}{
		{policy: "reverse", want: "12"},
		{policy: "declared", want: "12"},
	} {
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, mustPolicy(t, schedule.policy))
		result, err := ctx.RunAnalysis(sym, AnalysisArgs{}, m.idx.DocumentRoot(m.path), nil)
		if err != nil {
			t.Fatalf("%s analysis run: %v", schedule.policy, err)
		}
		if got := analysisResultValue(t, ctx, result, "result"); got != schedule.want {
			t.Errorf("%s result = %s, want %s", schedule.policy, got, schedule.want)
		}
	}
}

func TestAnalysisCaseStepOrderCheckOracle(t *testing.T) {
	m := caseStepOrderModel(t, "analysis_explore_step_order")
	want := loadCheckExpected(t, "analysis_explore_step_order")
	report := checkStart(t, m, starterOf(m.action(t, "Use")),
		CheckOptions{Reduce: true, Diverge: []string{"r"}})
	if got := report.Verdict.String(); got != want.Verdict {
		t.Fatalf("check verdict = %s (%+v), want %s", got, report, want.Verdict)
	}
	for feature, values := range want.Divergent {
		if got := divergentValues(report, feature); !slices.Equal(got, values) {
			t.Errorf("%s diverges over %v, want %v", feature, got, values)
		}
	}
}

func TestAnalysisCaseStepOrderCommutingAndStatedFlow(t *testing.T) {
	for _, tc := range []struct {
		name string
		fqn  string
		want []string
		runs int
	}{
		{name: "analysis_case_step_order_commuting", fqn: "test::Commute", want: []string{"3"}, runs: 2},
		{name: "analysis_case_step_order_stated", fqn: "test::Ordered", want: []string{"12"}, runs: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := caseStepOrderModel(t, tc.name)
			sym := namedOrFoundSymbol(t, m.idx, tc.fqn, m.idx.DocumentRoot(m.path), ast.DefAnalysisCase, ast.UsageAnalysisCase)
			exploration := exploreCaseAnalysis(t, m, sym)
			if !exploration.Complete() || exploration.Runs != tc.runs {
				t.Fatalf("exploration %s after %d runs, want %d complete run(s)",
					exploration.Status(), exploration.Runs, tc.runs)
			}
			if got := caseResultValues(t, exploration, "result"); !slices.Equal(got, tc.want) {
				t.Fatalf("analysis outcomes are %v, want %v", got, tc.want)
			}
			t.Logf("exploration reached %d outcome(s) in %d run(s)", len(exploration.Outcomes), exploration.Runs)
		})
	}
}

func TestVerificationCaseStepOrderOutcomes(t *testing.T) {
	m := caseStepOrderModel(t, "verification_explore_step_order")
	sym := namedOrFoundSymbol(t, m.idx, "test::OrderCheck", m.idx.DocumentRoot(m.path), ast.DefVerificationCase, ast.UsageVerificationCase)
	policy, err := ExplorePolicy(DefaultExploreBudget)
	if err != nil {
		t.Fatal(err)
	}
	exploration, err := Explore(context.Background(), policy, m.fresh, func(ctx *Context) (Outcome, error) {
		result, err := ctx.RunVerification(sym, AnalysisArgs{}, m.idx.DocumentRoot(m.path), nil)
		if err != nil {
			return Outcome{}, err
		}
		return Outcome{Outputs: map[string]Value{
			"verdict": NewStringValue(string(result.Verdict.Kind)),
		}}, nil
	})
	if err != nil {
		t.Fatalf("explore verification case: %v", err)
	}
	if !exploration.Complete() {
		t.Fatalf("verification exploration %s", exploration.Status())
	}
	got := make([]string, len(exploration.Outcomes))
	for i, outcome := range exploration.Outcomes {
		value, ok := outcome.Outcome.Outputs["verdict"]
		if !ok {
			t.Fatalf("verification outcome %s has no verdict", outcome.Outcome)
		}
		got[i] = FormatValue(value)
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{`"fail"`, `"pass"`}) {
		t.Fatalf("verification verdicts are %v, want [fail pass]", got)
	}
}

func TestVerificationCaseStepOrderCheckFindsViolation(t *testing.T) {
	m := caseStepOrderModel(t, "verification_explore_step_order")
	property := CheckProperty{
		Name: "verification case passes",
		Holds: func(_ *Context, inv *Invocation) (bool, error) {
			value, ok := inv.Actions[0].Results()["verdict"]
			if !ok {
				return true, nil
			}
			return FormatValue(value) == "VerdictKind::pass", nil
		},
	}
	report := checkStart(t, m, starterOf(m.action(t, "Harness")), reduced(), property)
	for _, violation := range report.Violations {
		if violation.Kind == ViolationProperty && violation.Name == property.Name {
			return
		}
	}
	t.Fatalf("check %+v found no violation of the order-dependent verification verdict", report)
}

func exploreCaseAnalysis(t *testing.T, m *exploreModel, sym *symbols.Symbol) *Exploration {
	t.Helper()
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
		t.Fatalf("explore analysis case: %v", err)
	}
	return exploration
}

func caseStepOrderModel(t *testing.T, name string) *exploreModel {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "conformance", name+".sysml"))
	if err != nil {
		t.Fatal(err)
	}
	return parseLibraryModel(t, string(data))
}

func caseResultValues(t *testing.T, exploration *Exploration, name string) []string {
	t.Helper()
	values := make([]string, len(exploration.Outcomes))
	for i, outcome := range exploration.Outcomes {
		values[i] = outcomeValue(t, outcome.Outcome, name)
	}
	slices.Sort(values)
	return values
}

func analysisResultValue(t *testing.T, ctx *Context, result AnalysisResult, name string) string {
	t.Helper()
	for _, output := range result.Outputs {
		if output.Name == name {
			return FormatValue(output.Value)
		}
	}
	t.Fatalf("analysis result has no output %q", name)
	return ""
}
