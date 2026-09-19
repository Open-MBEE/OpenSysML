package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// specializationModel is the part of the semantic model the resolver asks for
// when it follows specialization edges. semantics.Model itself imports resolve,
// so these tests supply the same edges the model derives: the resolved targets
// of the declared generalization relationships.
type specializationModel struct{ r *Resolver }

func (m specializationModel) DirectSupertypes(sym *symbols.Symbol) []*symbols.Symbol {
	var rels []*ast.Relationship
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		rels = d.Relationships
	case *ast.Usage:
		rels = d.Relationships
	default:
		return nil
	}
	var out []*symbols.Symbol
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines, ast.RelTyping:
		default:
			continue
		}
		target, ok := rel.Target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		if sup, ok := m.r.ResolveQualified(sym.OwnerScope, target); ok && sup != sym {
			out = append(out, sup)
		}
	}
	return out
}

func (m specializationModel) LookupMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if sym.Scope != nil {
		if found, ok := sym.Scope.LookupLocal(name); ok {
			return found, true
		}
	}
	return m.LookupContributedMember(sym, name)
}

func (m specializationModel) LookupContributedMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	seen := map[*symbols.Symbol]bool{sym: true}
	queue := m.DirectSupertypes(sym)
	for len(queue) > 0 {
		sup := queue[0]
		queue = queue[1:]
		if seen[sup] {
			continue
		}
		seen[sup] = true
		if sup.Scope != nil {
			if found, ok := sup.Scope.LookupLocal(name); ok {
				return found, true
			}
		}
		queue = append(queue, m.DirectSupertypes(sup)...)
	}
	return nil, false
}

// resolverWithSpecializations returns a resolver that knows the specialization
// edges of the indexed documents, as a live workspace's does.
func resolverWithSpecializations(idx *symbols.Index) *Resolver {
	r := New(idx)
	r.SetModel(specializationModel{r: r})
	return r
}

const protectedLib = `package Lib { public part def Pub; private part def Sec; }`

// A protected import is visible in the importing definition and in what
// specializes it (SysML v2 7.5.3).
func TestProtectedImportReachesADirectSpecialization(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": protectedLib,
		"app.sysml": `package App {
			part def Base { protected import Lib::*; }
			part def Sub :> Base;
		}`,
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := resolverWithSpecializations(idx)
	if _, ok := r.ResolveName(scopeOf(t, app, "Base"), "Pub", ident("Pub")); !ok {
		t.Fatalf("Pub is not visible in the importing body; diags=%v", r.Diagnostics)
	}
	r2 := resolverWithSpecializations(idx)
	if _, ok := r2.ResolveName(scopeOf(t, app, "Sub"), "Pub", ident("Pub")); !ok {
		t.Fatalf("Pub is not visible in Sub, which specializes the importing Base; "+
			"diags=%v", r2.Diagnostics)
	}
}

// The reach follows the whole specialization chain, not just its first edge.
func TestProtectedImportReachesATransitiveSpecialization(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": protectedLib,
		"app.sysml": `package App {
			part def Base { protected import Lib::*; }
			part def Mid :> Base;
			part def Leaf :> Mid;
		}`,
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := resolverWithSpecializations(idx)
	if _, ok := r.ResolveName(scopeOf(t, app, "Leaf"), "Pub", ident("Pub")); !ok {
		t.Fatalf("Pub is not visible in Leaf :> Mid :> Base; diags=%v", r.Diagnostics)
	}
}

// A feature typing is a generalization edge, so an import declared in a
// definition reaches the body of a usage typed by it.
func TestProtectedImportReachesAUsageTypedByTheImporter(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": protectedLib,
		"app.sysml": `package App {
			part def Base { protected import Lib::*; }
			part b : Base;
		}`,
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := resolverWithSpecializations(idx)
	if _, ok := r.ResolveName(scopeOf(t, app, "b"), "Pub", ident("Pub")); !ok {
		t.Fatalf("Pub is not visible in a usage typed by the importing Base; diags=%v",
			r.Diagnostics)
	}
}

// Protected reach stops at the specialization graph: an unrelated definition,
// and the enclosing package itself, see nothing.
func TestProtectedImportDoesNotReachAnUnrelatedNamespace(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": protectedLib,
		"app.sysml": `package App {
			part def Base { protected import Lib::*; }
			part def Other;
		}`,
		"far.sysml": `package Far { part def Alone; }`,
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := resolverWithSpecializations(idx)
	if sym, ok := r.ResolveName(scopeOf(t, app, "Other"), "Pub", ident("Pub")); ok {
		t.Fatalf("Pub resolved to %q in Other, which does not specialize Base", sym.Name)
	}
	r2 := resolverWithSpecializations(idx)
	if sym, ok := r2.ResolveName(app, "Pub", ident("Pub")); ok {
		t.Fatalf("Pub resolved to %q in App, which owns Base but does not specialize it",
			sym.Name)
	}
	far := scopeOf(t, idx.DocumentRoot("far.sysml"), "Far")
	r3 := resolverWithSpecializations(idx)
	if sym, ok := r3.ResolveName(scopeOf(t, far, "Alone"), "Pub", ident("Pub")); ok {
		t.Fatalf("Pub resolved to %q in an unrelated document", sym.Name)
	}
}

func TestProtectedImportReexportFollowsSpecialization(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": protectedLib,
		"app.sysml": `package App {
			part def Base { protected import Lib::Pub; }
			part def Sub :> Base { public import Base::*; }
			part def Other { public import Base::*; }
		}`,
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := resolverWithSpecializations(idx)
	if _, ok := r.ResolveName(scopeOf(t, app, "Sub"), "Pub", ident("Pub")); !ok {
		t.Fatalf("Pub is not visible through Sub's re-export; diags=%v", r.Diagnostics)
	}
	r2 := resolverWithSpecializations(idx)
	if _, ok := r2.ResolveName(scopeOf(t, app, "Other"), "Pub", ident("Pub")); ok {
		t.Fatal("Pub must not be re-exported through an unrelated namespace")
	}
}

// A private import keeps its narrower reach: it is visible in the body that
// declares it and in nothing that specializes that body.
func TestPrivateImportDoesNotReachASpecialization(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": protectedLib,
		"app.sysml": `package App {
			part def Base { private import Lib::*; }
			part def Sub :> Base;
		}`,
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := resolverWithSpecializations(idx)
	if _, ok := r.ResolveName(scopeOf(t, app, "Base"), "Pub", ident("Pub")); !ok {
		t.Fatalf("Pub must stay visible in the body declaring the private import; "+
			"diags=%v", r.Diagnostics)
	}
	r2 := resolverWithSpecializations(idx)
	if sym, ok := r2.ResolveName(scopeOf(t, app, "Sub"), "Pub", ident("Pub")); ok {
		t.Fatalf("Pub resolved to %q in Sub through a private import", sym.Name)
	}
}

// Visibility governs the reach of an import; `import all` governs which of the
// target's members it surfaces. A protected `import all` therefore carries the
// target's private members into specializations, and a protected plain import
// does not.
func TestProtectedImportAllReachesASpecializationWithPrivateMembers(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": protectedLib,
		"app.sysml": `package App {
			part def Base { protected import all Lib::*; }
			part def Sub :> Base;
			part def Plain { protected import Lib::*; }
			part def PlainSub :> Plain;
		}`,
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := resolverWithSpecializations(idx)
	if _, ok := r.ResolveName(scopeOf(t, app, "Sub"), "Sec", ident("Sec")); !ok {
		t.Fatalf("a protected `import all` must carry the private Sec into Sub; "+
			"diags=%v", r.Diagnostics)
	}
	r2 := resolverWithSpecializations(idx)
	if sym, ok := r2.ResolveName(scopeOf(t, app, "PlainSub"), "Sec", ident("Sec")); ok {
		t.Fatalf("private Sec resolved to %q through a plain protected import", sym.Name)
	}
}

// An Expose is a protected import (SysML v2 8.3.26.2), so what a view exposes is
// visible in a view that specializes it.
func TestExposeReachesASpecializingView(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": protectedLib,
		"app.sysml": `package App {
			view def V { expose Lib::*; }
			view def W :> V;
			view w : V;
		}`,
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := resolverWithSpecializations(idx)
	if _, ok := r.ResolveName(scopeOf(t, app, "W"), "Pub", ident("Pub")); !ok {
		t.Fatalf("an exposed element is not visible in the specializing view def W; "+
			"diags=%v", r.Diagnostics)
	}
	r2 := resolverWithSpecializations(idx)
	if _, ok := r2.ResolveName(scopeOf(t, app, "w"), "Pub", ident("Pub")); !ok {
		t.Fatalf("an exposed element is not visible in the view usage w : V; diags=%v",
			r2.Diagnostics)
	}
}

// A specialization cycle must end the walk rather than recurse forever.
func TestInheritedImportWalkTerminatesOnASpecializationCycle(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": protectedLib,
		"app.sysml": `package App {
			part def A :> B { protected import Lib::*; }
			part def B :> A;
		}`,
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := resolverWithSpecializations(idx)
	if _, ok := r.ResolveName(scopeOf(t, app, "B"), "Pub", ident("Pub")); !ok {
		t.Fatalf("Pub is not visible in B :> A; diags=%v", r.Diagnostics)
	}
}
