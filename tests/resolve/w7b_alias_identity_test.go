package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// aliasWorkspace resolves a library of aliases and a client importing one of
// them, returning the resolver and the client's scope.
func aliasWorkspace(t *testing.T) (*resolve.Resolver, *symbols.Scope) {
	t.Helper()
	idx := symbols.NewIndex()
	for name, src := range map[string]string{
		"defs.sysml": `package defs {
	part def Vehicle;
	alias Car for Vehicle;
	alias Auto for Car;
}`,
		"app.sysml": `package App {
	private import defs::Car;
	private import defs::Auto;
}`,
	} {
		p := parser.New(source.New(name, []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("%s parse diagnostics: %v", name, p.Diagnostics)
		}
		idx.AddDocument(name, root)
	}
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.SetModel(semantics.NewModel(r))
	app := idx.LookupQualified("App")
	if len(app) != 1 || app[0].Scope == nil {
		t.Fatalf("App names %d symbols with a scope", len(app))
	}
	return r, app[0].Scope
}

// A membership import of an alias imports the alias membership (KerML §8.2.3.2),
// so the name the client offers is the alias's and not the target's.
func TestW7BImportOfAnAliasMembershipNamesTheMembership(t *testing.T) {
	r, scope := aliasWorkspace(t)
	imports := importsOf(t, scope)
	if len(imports) != 2 {
		t.Fatalf("the client declares %d imports, want 2", len(imports))
	}
	for i, want := range [][]string{{"Car"}, {"Auto"}} {
		if got := namesOf(r.ImportedElements(scope, imports[i])); !equalNames(got, want) {
			t.Errorf("import %d surfaces %v, want %v", i, got, want)
		}
	}
}

// Every link of an alias chain names one element, so a reference through any of
// them denotes the same element as a reference to the target itself.
func TestW7BAliasChainDenotesOneElement(t *testing.T) {
	r, scope := aliasWorkspace(t)
	resolved := map[string]*symbols.Symbol{}
	for _, name := range []string{"Car", "Auto"} {
		sym, ok := r.ResolveQualified(scope, qualifiedName(name))
		if !ok {
			t.Fatalf("%s unresolved; diags=%v", name, r.Diagnostics)
		}
		resolved[name] = sym
	}
	target, ok := r.ResolveQualified(scope, qualifiedName("defs", "Vehicle"))
	if !ok {
		t.Fatalf("defs::Vehicle unresolved; diags=%v", r.Diagnostics)
	}
	for name, sym := range resolved {
		if !symbols.SameElement(sym, target) {
			t.Errorf("%s resolves to %s, which is not the element defs::Vehicle", name, symbols.FQNOf(sym))
		}
	}
	if !symbols.SameElement(resolved["Car"], resolved["Auto"]) {
		t.Error("two names for one element must denote the same element")
	}
}
