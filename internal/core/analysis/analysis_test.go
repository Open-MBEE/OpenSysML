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
// sweep, and an action whose three writers race.
const fixtureModel = `package test {
	calc def Double { in x : Integer; return : Integer = x * 2; }
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

// fresh is a runtime of a run's own over the model.
func (f *fixture) fresh() (*runtime.Context, error) {
	return runtime.NewContext(f.model, f.resolver, 10000), nil
}

// context is a runtime the surface would hold over the model.
func (f *fixture) context(t *testing.T) *runtime.Context {
	t.Helper()
	ctx, err := f.fresh()
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	return ctx
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
