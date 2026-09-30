package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// resolveActionBody parses, indexes and resolves src, returning the resolver,
// the scope of the action definition A it declares and A's succession edges.
func resolveActionBody(t *testing.T, src string) (*Resolver, *symbols.Scope, []*ast.SuccessionEdge) {
	t.Helper()
	const name = "a.sysml"
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	r := New(idx)
	r.ResolveDocument(name, root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("resolve diagnostics: %v", r.Diagnostics)
	}
	a, ok := idx.DocumentRoot(name).LookupLocal("A")
	if !ok {
		t.Fatalf("A is not declared")
	}
	def := root.Members[0].(*ast.Membership).Member.(*ast.Definition)
	var edges []*ast.SuccessionEdge
	for _, m := range def.Members {
		if edge, ok := m.(*ast.SuccessionEdge); ok {
			edges = append(edges, edge)
		}
	}
	return r, a.Scope, edges
}

// The bare `then fork;` beside a member named `fork` references that member,
// and the reference resolves to it.
func TestBareThenNodeWordReferencesTheDeclaredMember(t *testing.T) {
	r, scope, edges := resolveActionBody(t, "action def A { action fork; action a; then fork; }")
	if len(edges) != 1 || edges[0].Target == nil {
		t.Fatalf("want one succession with a named target, got %v", edges)
	}
	got, ok := r.PartSymbol(edges[0].Target, 0)
	want, _ := scope.LookupLocal("fork")
	if !ok || got != want {
		t.Fatalf("`then fork;` resolves to %v, want the member `fork` %v", got, want)
	}
}

// `then fork F;` declares the fork node F beside a member named `fork` as it
// does in a body declaring none, and the edge's target resolves to that node.
func TestNamedThenNodeDeclaresTheNodeBesideADeclaredMember(t *testing.T) {
	for name, src := range map[string]string{
		"without a member":       "action def A { action a; then fork F; }",
		"beside a member":        "action def A { action fork; action a; then fork F; }",
		"beside a quoted member": "action def A { action 'fork'; action a; then fork F; }",
		"after a multiplicity":   "action def A { action fork; action a; then [0..1] fork F; }",
	} {
		t.Run(name, func(t *testing.T) {
			r, scope, edges := resolveActionBody(t, src)
			f, ok := scope.LookupLocal("F")
			if !ok {
				t.Fatalf("F is not declared in A")
			}
			if _, ok := f.Decl.(*ast.ForkNode); !ok {
				t.Fatalf("F is declared by a %T, want *ast.ForkNode", f.Decl)
			}
			if len(edges) != 1 || edges[0].Target == nil {
				t.Fatalf("want one succession with a named target, got %v", edges)
			}
			if got, ok := r.PartSymbol(edges[0].Target, 0); !ok || got != f {
				t.Fatalf("the succession's target resolves to %v, want the fork node F %v", got, f)
			}
			if edges[0].SourceMultiplicity == nil && name == "after a multiplicity" {
				t.Fatalf("the multiplicity ahead of the declaration is not the source end's: %+v", edges[0])
			}
		})
	}
}
