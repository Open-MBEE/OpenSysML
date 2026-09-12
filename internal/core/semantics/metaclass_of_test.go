package semantics_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// metaclassModel indexes src over the standard library and resolves it.
func metaclassModel(t *testing.T, name, src string) (*semantics.Model, *symbols.Scope) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocumentWithKind(name, root, source.KindOf(name))
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)
	return m, idx.DocumentRoot(name)
}

// declarations walks every symbol declared under scope, depth first.
func declarations(scope *symbols.Scope, visit func(*symbols.Symbol)) {
	for _, sym := range scope.AllMembers() {
		visit(sym)
	}
	for _, child := range scope.Children() {
		declarations(child, visit)
	}
}

// TestMetaclassOfEveryDeclaration: every declaration over the standard library has a
// reflective metaclass, connector ends, bindings, transitions and satisfies included.
func TestMetaclassOfEveryDeclaration(t *testing.T) {
	models := map[string]string{
		"t.sysml": `package T {
			port def P;
			part def Wheel;
			part def Car {
				part w : Wheel[4];
				port p : P;
				connection c connect a references w to z references p;
				binding b bind w = w;
				end e [0..1] part x : Wheel : Wheel;
				interface i connect a references p to z references p;
				state def S { state a; state z; transition first a then z; }
				requirement def R;
				satisfy requirement R by w;
			}
			connection def Link { end part a : Wheel; end part b : Wheel; }
		}`,
		"t.kerml": `package K {
			class Wheel;
			class Car {
				feature w : Wheel[4];
				connector c from a references w to z references w;
				end e [0..1] feature x : Wheel : Wheel;
			}
			assoc Link { end feature a : Wheel; end feature b : Wheel; }
		}`,
	}
	for name, src := range models {
		m, root := metaclassModel(t, name, src)
		declarations(root, func(sym *symbols.Symbol) {
			if _, isRoot := sym.Decl.(*ast.RootNamespace); isRoot {
				return
			}
			if m.MetaclassOf(sym) == nil {
				t.Errorf("%s: %s %q (%T) has no metaclass", name, sym.Kind, sym.Name, sym.Decl)
			}
		})
	}
}

// TestMetaclassOfRefinedKinds pins the metaclass of the symbol kinds that span
// several metaclasses, as the pilot grammars declare them.
func TestMetaclassOfRefinedKinds(t *testing.T) {
	m, root := metaclassModel(t, "t.sysml", `package T {
		port def P;
		part def Wheel;
		part def Car {
			part w : Wheel;
			port p : P;
			connection c connect a references w to z references p;
			interface i connect a references p to z references p;
			binding b bind w = w;
			state def S { state a; state z; transition t first a then z; }
			requirement def R;
			satisfy requirement r : R by w;
			end e [0..1] part x : Wheel : Wheel;
		}
	}`)
	find := func(kind symbols.SymbolKind, owner string) *symbols.Symbol {
		var found *symbols.Symbol
		declarations(root, func(sym *symbols.Symbol) {
			if found == nil && sym.Kind == kind && sym.OwnerScope != nil && sym.OwnerScope.Owner() != nil && sym.OwnerScope.Owner().Name == owner {
				found = sym
			}
		})
		if found == nil {
			t.Fatalf("no %s under %s", kind, owner)
		}
		return found
	}
	byName := func(name string) *symbols.Symbol {
		var found *symbols.Symbol
		declarations(root, func(sym *symbols.Symbol) {
			if found == nil && sym.Name == name {
				found = sym
			}
		})
		if found == nil {
			t.Fatalf("no symbol %q", name)
		}
		return found
	}
	cases := []struct {
		sym  *symbols.Symbol
		want string
	}{
		{find(symbols.SymbolConnectorEnd, "c"), "SysML::Systems::ReferenceUsage"},
		{find(symbols.SymbolConnectorEnd, "i"), "SysML::Systems::PortUsage"},
		{byName("b"), "SysML::Systems::BindingConnectorAsUsage"},
		{byName("t"), "SysML::Systems::TransitionUsage"},
		{byName("r"), "SysML::Systems::SatisfyRequirementUsage"},
		{byName("e"), "SysML::Systems::ReferenceUsage"},
	}
	for _, tc := range cases {
		meta := m.MetaclassOf(tc.sym)
		if meta == nil {
			t.Errorf("%s %q: no metaclass, want %s", tc.sym.Kind, tc.sym.Name, tc.want)
			continue
		}
		if got := symbols.FQNOf(meta); got != tc.want {
			t.Errorf("%s %q: metaclass %s, want %s", tc.sym.Kind, tc.sym.Name, got, tc.want)
		}
	}
}
