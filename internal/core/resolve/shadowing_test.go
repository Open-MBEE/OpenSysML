package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestGeneralizationHeaderResolvesOutsideTheDeclarationBody(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		class A { class a1; }
		class B specializes A {
			class A { class a2; }
			class b specializes a1;
		}
		feature C { feature c1; }
		feature D subsets C {
			feature C { feature c2; }
			feature d redefines C::c1;
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("generalization headers resolved in their bodies: %v", r.Diagnostics)
	}
}

func TestQualifiedRedefinitionReportsOneUnresolvedDiagnostic(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		feature C { feature c1; }
		feature D subsets C {
			feature d redefines C::nope;
		}
	}`)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want one unresolved target", r.Diagnostics)
	}
}

// A qualified redefinition target never starts at a sibling of the redefining
// feature (KerML 8.2.3.5.2): the general D subsets has no nope, and the outer
// C has none either, so the one diagnostic is the unresolved target itself.
func TestQualifiedRedefinitionFallbackDoesNotReportSpeculativeFailure(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		feature C { feature c1; }
		feature D subsets C {
			feature C { feature nope; }
			feature d redefines C::nope;
		}
	}`)
	if len(r.Diagnostics) != 1 || !strings.HasPrefix(r.Diagnostics[0].Message, "unresolved reference: C::nope") {
		t.Fatalf("diagnostics = %v, want one unresolved C::nope", r.Diagnostics)
	}
}

func TestEnclosingDeclarationShadowsANestedImport(t *testing.T) {
	idx := symbols.NewIndex()
	idx.AddDocument("lib.kerml", parsedRoot(t, "lib.kerml", `package Lib {
		feature A { feature a1; }
	}`))
	root := parsedRoot(t, "app.kerml", `package P {
		feature A { feature a2; }
		feature direct {
			public import Lib::*;
			feature B redefines A { feature b redefines a2; }
		}
		feature inheritedSource { public import Lib::*; }
		feature inherited subsets inheritedSource {
			feature B redefines A { feature b redefines a1; }
		}
	}`)
	idx.AddDocument("app.kerml", root)
	idx.ExpandWildcardImports()
	r := resolverWithSpecializations(idx)
	r.ResolveDocument("app.kerml", root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("nested import shadowed the enclosing declaration: %v", r.Diagnostics)
	}
}

// Two roots of the same name are one global name, and the tail of a qualified
// reference continues in the root it resolved to (reference fixture
// DependencySamePackageName).
func TestRepeatedRootNameResolvesToOneRootWithoutAmbiguity(t *testing.T) {
	idx := symbols.NewIndex()
	idx.AddDocument("a.kerml", parsedRoot(t, "a.kerml", `package Same {
		feature container { feature a; }
	}`))
	idx.AddDocument("b.kerml", parsedRoot(t, "b.kerml", `package Same {
		feature container { feature b; }
	}`))
	root := parsedRoot(t, "app.kerml", `package P {
		feature x : Same::container::a;
	}`)
	idx.AddDocument("app.kerml", root)
	idx.ExpandWildcardImports()
	r := resolverWithSpecializations(idx)
	r.ResolveDocument("app.kerml", root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("repeated root name reported: %v", r.Diagnostics)
	}
}
