package explore_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/explore"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func mustPolicy(t *testing.T, spelling string) runtime.SchedulePolicy {
	t.Helper()
	policy, err := runtime.ParseSchedulePolicy(spelling)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func oneSymbol(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", fqn, len(matches))
	}
	return matches[0]
}

func namedOrFoundSymbol(t *testing.T, idx *symbols.Index, fqn string, scope *symbols.Scope, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
	if fqn == "" {
		return findBehavioralSymbol(t, scope, defKind, usageKind)
	}
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("evaluate %q: %d matching symbols, want 1", fqn, len(matches))
	}
	if !behavioralKind(matches[0], defKind, usageKind) {
		t.Fatalf("evaluate %q: names a %T, want a %v/%v", fqn, matches[0].Decl, defKind, usageKind)
	}
	return matches[0]
}

func findBehavioralSymbol(t *testing.T, scope *symbols.Scope, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
	sym := lookupBehavioralSymbol(scope, defKind, usageKind)
	if sym == nil {
		t.Fatalf("no behavioral symbol found (defKind=%v, usageKind=%v)", defKind, usageKind)
	}
	return sym
}

func behavioralKind(sym *symbols.Symbol, defKind ast.DefinitionKind, usageKind ast.UsageKind) bool {
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		return decl.Kind == defKind
	case *ast.Usage:
		return decl.Kind == usageKind
	default:
		return false
	}
}

func lookupBehavioralSymbol(scope *symbols.Scope, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
	// Check all child scopes (packages/namespaces)
	for _, child := range scope.Children() {
		// Look for named symbols
		for _, name := range child.MemberNames() {
			sym, ok := child.LookupLocal(name)
			if !ok {
				continue
			}
			if def, ok := sym.Decl.(*ast.Definition); ok && def.Kind == defKind {
				return sym
			}
			if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == usageKind {
				return sym
			}
		}
	}

	// Also check root scope directly
	for _, name := range scope.MemberNames() {
		sym, ok := scope.LookupLocal(name)
		if !ok {
			continue
		}
		if def, ok := sym.Decl.(*ast.Definition); ok && def.Kind == defKind {
			return sym
		}
		if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == usageKind {
			return sym
		}
	}

	return nil
}

func starterOf(sym *symbols.Symbol) runtime.Starter {
	return func(ctx *runtime.Context) (*runtime.Invocation, error) {
		exec, err := ctx.CreateActionExecutor(sym)
		if err != nil {
			return nil, err
		}
		return &runtime.Invocation{Actions: []*runtime.ActionExecutor{exec}}, nil
	}
}

func reduced() runtime.CheckOptions   { return runtime.CheckOptions{Reduce: true} }
func unreduced() runtime.CheckOptions { return runtime.CheckOptions{} }

func divergentValues(report *runtime.CheckReport, feature string) []string {
	for _, d := range report.Divergent {
		if d.Feature == feature {
			values := make([]string, len(d.Values))
			for i, v := range d.Values {
				values[i] = v.Value
			}
			return values
		}
	}
	return nil
}

func replayed(t *testing.T, fresh func() (*runtime.Context, error), run func(*runtime.Context) (runtime.Outcome, error), witness []runtime.ChoiceTaken) (runtime.Outcome, []runtime.ChoiceTaken, error) {
	t.Helper()
	ctx, err := fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, runtime.ReplayPolicy(witness))
	outcome, err := run(ctx)
	if err == nil {
		err = ctx.Unfollowed()
	}
	var choices []runtime.ChoiceTaken
	for _, c := range ctx.Choices() {
		choices = append(choices, c.Choice())
	}
	return outcome, choices, err
}

func assertWitnessesReplay(t *testing.T, x *explore.Exploration, fresh func() (*runtime.Context, error), run func(*runtime.Context) (runtime.Outcome, error)) {
	t.Helper()
	for _, o := range x.Outcomes {
		outcome, choices, err := replayed(t, fresh, run, o.Witness)
		if err != nil {
			t.Errorf("%s: replaying %s: %v", o.Outcome, runtime.FormatChoices(o.Witness), err)
			continue
		}
		if outcome.String() != o.Outcome.String() {
			t.Errorf("replaying %s reached %s, want %s", runtime.FormatChoices(o.Witness), outcome, o.Outcome)
		}
		if got, want := runtime.FormatChoices(choices), runtime.FormatChoices(o.Witness); got != want {
			t.Errorf("replaying %s made the choices\n%s", want, got)
		}
	}
}

func primeLibraryCache() error {
	src := libs.DefaultSource()
	cache, err := libs.NewCache()
	if err != nil {
		return err
	}
	if err := libs.NewLoader(src, cache).LoadAll(symbols.NewIndex()); err != nil {
		return fmt.Errorf("load the library: %w", err)
	}
	return nil
}

func buildRuntimeWithLibraries(t *testing.T, path string, file *ast.RootNamespace) (*symbols.Index, *semantics.Model, *runtime.Context) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument(path, file)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	return idx, model, runtime.NewContext(parsingModel(model, resolver), 10000)
}

func parsingModel(sem *semantics.Model, resolver *resolve.Resolver) *runtime.Model {
	m := typedModel(sem, resolver)
	m.SetExpressionParser(parser.ParseOneExpression)
	return m
}

func conformanceModel(t *testing.T, name string) *exploreModel {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("..", "runtime", "testdata", "conformance", name+".sysml"))
	if err != nil {
		t.Fatal(err)
	}
	return parseExploreModel(t, string(text))
}

// sendSignal enqueues a signal event that names a declared signal, as a test's
// run drives its state machine; a payload the machine cannot take is a bug in the test.
func sendSignal(exec *runtime.StateExecutor, name string, args map[string]runtime.Value) {
	if err := exec.Enqueue(runtime.QueuedEvent{Signal: name, Args: args}); err != nil {
		panic(err)
	}
}

func replayWitness(t *testing.T, m *exploreModel, start runtime.Starter, w runtime.Witness, claim string, props ...runtime.CheckProperty) *runtime.Replayed {
	t.Helper()
	parsed, err := runtime.ParseWitness(w.String())
	if err != nil {
		t.Fatalf("%s: parsing the witness: %v", claim, err)
	}
	if parsed.Trace != w.Trace || parsed.Fails != w.Fails || parsed.Property != w.Property || len(parsed.Choices) != len(w.Choices) {
		t.Fatalf("%s: the witness reads back otherwise:\n%s", claim, w)
	}
	r, err := runtime.Replay(context.Background(), m.fresh, start, parsed, props)
	if err != nil {
		t.Fatalf("%s: replay: %v", claim, err)
	}
	return r
}

func finalOutcomes(report *runtime.CheckReport) []string {
	outcomes := make([]string, len(report.Finals))
	for i, f := range report.Finals {
		outcomes[i] = f.Outcome
	}
	sort.Strings(outcomes)
	return outcomes
}
