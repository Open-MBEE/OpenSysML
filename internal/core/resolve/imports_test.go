package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestImportMembership(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Widget; }",
		"b.sysml": "package App { import Lib::Widget; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	sym, ok := r.ResolveName(appScope, "Widget", &ast.FeatureReference{})
	if !ok {
		t.Fatalf("Widget unresolved via membership import; diags=%v", r.Diagnostics)
	}
	if sym.Name != "Widget" {
		t.Fatalf("resolved %q, want Widget", sym.Name)
	}
}

func TestMembershipImportAcceptsDeclaredAndShortNames(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package SI { attribute <kg> kilogram; attribute <g> gram; }",
		"app.sysml": "package App { import SI::kilogram; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	for _, name := range []string{"kilogram", "kg"} {
		if _, ok := r.ResolveName(appScope, name, ident(name)); !ok {
			t.Fatalf("%s unresolved via membership import; diags=%v", name, r.Diagnostics)
		}
	}
	if _, ok := r.ResolveName(appScope, "g", ident("g")); ok {
		t.Fatal("gram's short name must not resolve through an import of kilogram")
	}
}

func TestRootImportVisibleInNestedPackage(t *testing.T) {
	for _, ext := range []string{"sysml", "kerml"} {
		idx := indexOf(t, map[string]string{
			"base." + ext: "package Base { namespace Real; }",
			"app." + ext:  "private import Base::Real; package App { namespace Uses; }",
		})
		app := scopeOf(t, idx.DocumentRoot("app."+ext), "App")
		uses := scopeOf(t, app, "Uses")
		r := New(idx)
		if _, ok := r.ResolveName(uses, "Real", ident("Real")); !ok {
			t.Fatalf("%s: root import was not visible in nested package; diags=%v", ext, r.Diagnostics)
		}
	}
}

func TestImportPrefixResolvesThroughSiblingImport(t *testing.T) {
	for _, ext := range []string{"sysml", "kerml"} {
		idx := indexOf(t, map[string]string{
			"base." + ext: "package Occurrences { namespace Occurrence { namespace Coincident; } }",
			"app." + ext:  "package R8 { private import Occurrences::Occurrence; private import Occurrence::Coincident; }",
		})
		r8 := scopeOf(t, idx.DocumentRoot("app."+ext), "R8")
		r := New(idx)
		if _, ok := r.ResolveName(r8, "Occurrence", ident("Occurrence")); !ok {
			t.Fatalf("%s: sibling import did not expose Occurrence; diags=%v", ext, r.Diagnostics)
		}
		qualified := qn(false, "Occurrence", "Coincident")
		if _, ok := r.ResolveQualified(r8, qualified); !ok {
			t.Fatalf("%s: imported prefix did not resolve through sibling import; diags=%v", ext, r.Diagnostics)
		}
		if _, ok := r.ResolveName(r8, "Coincident", ident("Coincident")); !ok {
			t.Fatalf("%s: member imported through sibling prefix remained unresolved; diags=%v", ext, r.Diagnostics)
		}
	}
}

func TestImportNamespaceStar(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Widget; namespace Gadget; }",
		"b.sysml": "package App { import Lib::*; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	if _, ok := r.ResolveName(appScope, "Widget", &ast.FeatureReference{}); !ok {
		t.Fatalf("Widget unresolved via namespace import; diags=%v", r.Diagnostics)
	}
	if _, ok := r.ResolveName(appScope, "Gadget", &ast.FeatureReference{}); !ok {
		t.Fatalf("Gadget unresolved via namespace import")
	}
}

func TestImportRecursiveMembership(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Outer { namespace Deep; } }",
		"b.sysml": "package App { import Lib::Outer::**; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	if _, ok := r.ResolveName(appScope, "Outer", &ast.FeatureReference{}); !ok {
		t.Fatalf("Outer unresolved via recursive import; diags=%v", r.Diagnostics)
	}
	if _, ok := r.ResolveName(appScope, "Deep", &ast.FeatureReference{}); !ok {
		t.Fatalf("Deep unresolved via recursive import")
	}
}

func TestImportDoesNotLeakNonImported(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Widget; namespace Hidden; }",
		"b.sysml": "package App { import Lib::Widget; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	if _, ok := r.ResolveName(appScope, "Hidden", &ast.FeatureReference{}); ok {
		t.Fatalf("Hidden should NOT be visible (only Widget imported)")
	}
}

// An import owned by a definition body is a Namespace-owned Import and must be
// consulted when resolving names in that body.
func TestImportInDefinitionBodyVisibleInBody(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { part def Widget; }",
		"b.sysml": "package App { part def D { private import Lib::*; part w : Widget; } }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	dScope := scopeOf(t, appScope, "D")
	if _, ok := r.ResolveName(dScope, "Widget", &ast.FeatureReference{}); !ok {
		t.Fatalf("Widget unresolved from definition body that imports it; diags=%v", r.Diagnostics)
	}
}

// The import must also reach scopes nested inside the definition body.
func TestImportInDefinitionBodyVisibleInNestedBody(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { part def Widget; }",
		"b.sysml": "package App { part def D { private import Lib::*; action a { part w : Widget; } } }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	dScope := scopeOf(t, appScope, "D")
	aScope := scopeOf(t, dScope, "a")
	if _, ok := r.ResolveName(aScope, "Widget", &ast.FeatureReference{}); !ok {
		t.Fatalf("Widget unresolved from body nested in the importing definition; diags=%v", r.Diagnostics)
	}
}

// A package-body import must reach a definition nested inside that package
// (the pre-existing package case, exercised through a nested scope).
func TestImportInPackageBodyVisibleInNestedDefinition(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { part def Widget; }",
		"b.sysml": "package App { private import Lib::*; part def D { part w : Widget; } }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	dScope := scopeOf(t, appScope, "D")
	if _, ok := r.ResolveName(dScope, "Widget", &ast.FeatureReference{}); !ok {
		t.Fatalf("Widget unresolved from definition nested in importing package; diags=%v", r.Diagnostics)
	}
}

// An import owned by a definition body is a private membership of that body: it
// must not be re-surfaced to a namespace that imports the outer definition.
func TestImportInDefinitionBodyDoesNotLeakToImporter(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { part def Widget; }",
		"b.sysml": "package App { part def D { private import Lib::*; } import D::*; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	if _, ok := r.ResolveName(appScope, "Widget", &ast.FeatureReference{}); ok {
		t.Fatalf("Widget was imported privately into D and must not leak to importers of D")
	}
}

func TestImportRecursiveSkipsBodyLocalNames(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": `package Lib {
			action def Sample {
				in attribute samples;
				assert constraint { samples->forAll { in bodyParam; bodyParam > 0 } }
				loop action charging { } until true;
				if true { action thenLocal; } else { action elseLocal; }
			}
		}`,
		"b.sysml": "package App { import Lib::**; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	for _, name := range []string{"bodyParam", "charging", "thenLocal", "elseLocal"} {
		if _, ok := r.ResolveName(appScope, name, &ast.FeatureReference{}); ok {
			t.Errorf("%s is body-local and must not be importable", name)
		}
	}
}

// A root-level wildcard import surfaces its names in the editor's own scope tree
// even when the document declares nothing else: the tree is identified by the
// document name stamped on it, which no member is left to carry.
func TestRootImportInDocumentDeclaringNothingElse(t *testing.T) {
	idx := symbols.NewIndex()
	idx.AddDocument("lib.sysml", parsedRoot(t, "lib.sysml", "package Lib { namespace Widget; }"))

	const name = "repl.sysml"
	root := parsedRoot(t, name, "import Lib::*;")
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()

	scope := symbols.Build(root)
	symbols.SetDocName(scope, name)

	r := New(idx)
	if _, ok := r.ResolveName(scope, "Widget", ident("Widget")); !ok {
		t.Fatalf("Widget unresolved through the document's own root import; diags=%v", r.Diagnostics)
	}
}
