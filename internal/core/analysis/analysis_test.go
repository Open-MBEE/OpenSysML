package analysis

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// fixtureModel is the model the adapter tests ask about: a calc to evaluate and
// sweep, a part with one holding and one violated constraint, and an action whose
// three writers race.
const fixtureModel = `package test {
	calc def Double { in x : Integer; return : Integer = x * 2; }
	part def Tank {
		attribute pressure = 40.0;
		assert constraint low { pressure < 100.0 }
		assert constraint high { pressure > 100.0 }
	}
	action race {
		attribute x : Integer = 0;
		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		action c { assign x := 3; }
		join sync;
		done;
		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first split then c;
		succession first a then sync;
		succession first b then sync;
		succession first c then sync;
		succession first sync then done;
	}
}`

// fixture is a model parsed once, from which every context is built.
type fixture struct {
	idx      *symbols.Index
	model    *semantics.Model
	resolver *resolve.Resolver
	pkg      *symbols.Scope
}

func parseFixture(t *testing.T) *fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "analysis.sysml")
	p := parser.New(source.New(path, []byte(fixtureModel)))
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	idx := symbols.NewIndex()
	idx.AddDocument(path, file)
	resolver := resolve.New(idx)
	pkg, ok := idx.DocumentRoot(path).LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return &fixture{idx: idx, model: semantics.NewModel(resolver), resolver: resolver, pkg: pkg.Scope}
}

// fixtureSteps is the step limit every runtime over the fixture is built with.
const fixtureSteps = 10000

// semantics is a worker's own resolver and semantic model over the shared index.
func (f *fixture) semantics() (*resolve.Resolver, *semantics.Model, error) {
	resolver := resolve.New(f.idx)
	return resolver, semantics.NewModel(resolver), nil
}

// fresh is a runtime of a run's own on a worker.
func (f *fixture) fresh(w *Worker) (*runtime.Context, error) {
	return runtime.NewContext(w.Model, w.Resolver, fixtureSteps), nil
}

// building is the model as a surface holding no context of its own supplies it.
func (f *fixture) building() *Model {
	return &Model{Semantics: f.semantics, Fresh: f.fresh}
}

// context is a runtime the surface would hold over the model.
func (f *fixture) context(t *testing.T) *runtime.Context {
	t.Helper()
	return runtime.NewContext(f.model, f.resolver, fixtureSteps)
}

// symbol is the named member of the test package.
func (f *fixture) symbol(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := f.pkg.LookupLocal(name)
	if !ok {
		t.Fatalf("%s not indexed", name)
	}
	return sym
}

// intOf is an integer as a runtime value carries it.
func intOf(n int64) runtime.Value {
	return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

// policy parses a scheduling policy as a flag spells it.
func policy(t *testing.T, spelling string) runtime.SchedulePolicy {
	t.Helper()
	p, err := runtime.ParseSchedulePolicy(spelling)
	if err != nil {
		t.Fatalf("policy %s: %v", spelling, err)
	}
	return p
}

// answered dispatches q under auto and fails the test on a fault.
func answered(t *testing.T, r *Registry, model *Model, q Question, budget Budget) Plan {
	t.Helper()
	plan, err := r.Answer(context.Background(), model, q, budget)
	if err != nil {
		t.Fatalf("answer %s %s: %v", q.Kind, q.Subject, err)
	}
	return plan
}

// BudgetOf fills Runs in the unit of the question's kind: an exploring policy's
// runs for outcomes, the sweep runs for a sweep, none for a run or a solve; Jobs
// is the surface's, DefaultJobs when the surface set none.
func TestBudgetOfFillsRunsByKind(t *testing.T) {
	limits := runtime.Budgets{MaxSteps: 70, MaxElements: 90, MaxSweepRuns: 40}
	exploring := policy(t, "explore:runs=6,depth=4")
	cases := []struct {
		kind   Kind
		policy runtime.SchedulePolicy
		want   Budget
	}{
		{Outcomes, exploring, Budget{Jobs: 3, Runs: 6, Depth: 4, Steps: 70, Memory: 90}},
		{Outcomes, runtime.DefaultSchedulePolicy, Budget{Jobs: 3, Steps: 70, Memory: 90}},
		{Sweep, exploring, Budget{Jobs: 3, Runs: 40, Depth: 4, Steps: 70, Memory: 90}},
		{Sweep, runtime.DefaultSchedulePolicy, Budget{Jobs: 3, Runs: 40, Steps: 70, Memory: 90}},
		{Evaluate, exploring, Budget{Jobs: 3, Depth: 4, Steps: 70, Memory: 90}},
		{Satisfiable, exploring, Budget{Jobs: 3, Depth: 4, Steps: 70, Memory: 90}},
	}
	for _, tc := range cases {
		if got := BudgetOf(limits, tc.policy, tc.kind, 3); got != tc.want {
			t.Errorf("BudgetOf(%s, %s) = %+v, want %+v", tc.kind, tc.policy, got, tc.want)
		}
	}
	if got := BudgetOf(limits, exploring, Outcomes, 0).Jobs; got != DefaultJobs() {
		t.Errorf("BudgetOf with no jobs set Jobs = %d, want DefaultJobs %d", got, DefaultJobs())
	}
}
