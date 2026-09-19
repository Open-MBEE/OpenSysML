package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// diagsIn parses src as the document name given — whose extension decides the
// language — and returns the diagnostics of one source.
func diagsIn(t *testing.T, name, src, diagSource string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument(name, root)
	var out []diag.Diagnostic
	for _, d := range Analyze(name, root, nil, idx) {
		if d.Source == diagSource {
			out = append(out, d)
		}
	}
	return out
}

// A KerML type declaration is a Type and may specialize one: the parser records
// it as a usage for want of a definition node, which is not a KerML notion.
func TestTypeCheckKerMLSpecializationClean(t *testing.T) {
	for _, src := range []string{
		"class Object; class Person specializes Object;",
		"struct Wheel; struct MyWheel specializes Wheel;",
		"behavior B; behavior C specializes B;",
		"class A { feature f; } class B specializes A;",
	} {
		if diags := diagsIn(t, "a.kerml", src, "type"); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// The same declarations in SysML are usages, where only a definition may
// specialize: the exemption is the document's language, not the check's demise.
func TestTypeCheckSysMLSpecializationStillFires(t *testing.T) {
	for _, src := range []string{
		"class Object; class Person specializes Object;",
		"struct Wheel; struct MyWheel specializes Wheel;",
	} {
		diags := diagsIn(t, "a.sysml", src, "type")
		if len(diags) != 1 {
			t.Fatalf("%s: expected one type diagnostic, got %v", src, diags)
		}
		if !strings.Contains(diags[0].Message, "only a definition may specialize") {
			t.Errorf("%s: got %q", src, diags[0].Message)
		}
	}
}

// A KerML specialization target must still be a type: a package is not one.
func TestTypeCheckKerMLSpecializesNonTypeStillFires(t *testing.T) {
	diags := diagsIn(t, "a.kerml", "package P; class C specializes P;", "type")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "may specialize only a type") {
		t.Errorf("got %q", diags[0].Message)
	}
}

// A KerML FeatureTyping's type is any Type, a Feature among them, so a feature
// may be typed by a feature (KerML 1.0 §8.3.4.4).
func TestTypeCheckKerMLTypingByFeatureClean(t *testing.T) {
	for _, src := range []string{
		"class A; feature y : A; feature yy : y;",
		"class A { feature f; } feature v : A { feature m redefines f; }",
	} {
		if diags := diagsIn(t, "a.kerml", src, "type"); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// In SysML a usage may only be typed by a definition, which a feature is not.
func TestTypeCheckSysMLTypingByFeatureStillFires(t *testing.T) {
	diags := diagsIn(t, "a.sysml", "class A; feature y : A; feature yy : y;", "type")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "type must be a definition") {
		t.Errorf("got %q", diags[0].Message)
	}
}

// A KerML typing target must still be a type: a package is not one.
func TestTypeCheckKerMLTypedByNonTypeStillFires(t *testing.T) {
	diags := diagsIn(t, "a.kerml", "package P; feature f : P;", "type")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "type must be a type") {
		t.Errorf("got %q", diags[0].Message)
	}
}

// An alias resolves to its target upstream, so a chain of them names the type.
func TestTypeCheckKerMLSpecializesAliasOfTypeClean(t *testing.T) {
	src := "class Object; alias O1 for Object; alias O2 for O1; class C specializes O2;"
	if diags := diagsIn(t, "a.kerml", src, "type"); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}
}

// An alias reaches the check unresolved only when it is cyclic, naming no type.
func TestTypeCheckKerMLSpecializesCyclicAliasStillFires(t *testing.T) {
	for _, src := range []string{
		"alias A for A; class C specializes A;",
		"alias A for B; alias B for A; class C specializes A;",
	} {
		diags := diagsIn(t, "a.kerml", src, "type")
		if len(diags) != 1 {
			t.Fatalf("%s: expected one type diagnostic, got %v", src, diags)
		}
		if !strings.Contains(diags[0].Message, "may specialize only a type, found alias") {
			t.Errorf("%s: got %q", src, diags[0].Message)
		}
	}
}

// isTypeKind enumerates the types, so an unclassified kind is not one: a kind
// added later is rejected until someone classifies it.
func TestIsTypeKindIsAnAllowlist(t *testing.T) {
	for _, k := range []symbols.SymbolKind{
		symbols.SymbolKerMLType, symbols.SymbolPartDef, symbols.SymbolMetaclass,
		symbols.SymbolAttributeUsage, symbols.SymbolConnectorEnd,
		symbols.SymbolSatisfyRequirementUsage,
	} {
		if !isTypeKind(k) {
			t.Errorf("%s: expected a type", k)
		}
	}
	for _, k := range []symbols.SymbolKind{
		symbols.SymbolPackage, symbols.SymbolNamespace, symbols.SymbolAlias,
		symbols.SymbolDependency, symbols.SymbolComment, symbols.SymbolDocumentation,
		symbols.SymbolTextualRepresentation,
	} {
		if isTypeKind(k) {
			t.Errorf("%s: expected not a type", k)
		}
	}
	if n := symbols.SymbolSatisfyRequirementUsage + 1; isTypeKind(n) {
		t.Errorf("an unenumerated kind (%d) must not be a type", n)
	}
}

// A metaclass is a Class and specializes a metaclass, in either language.
func TestTypeCheckMetaclassSpecializesMetaclass(t *testing.T) {
	for _, name := range []string{"a.kerml", "a.sysml"} {
		src := "metaclass Metaobject; metaclass AtomMetadata specializes Metaobject;"
		if diags := diagsIn(t, name, src, "type"); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", name, diags)
		}
	}
}

// A metaclass may not specialize a definition of an unrelated kind.
func TestTypeCheckMetaclassSpecializesPartDefStillFires(t *testing.T) {
	diags := diagsIn(t, "a.sysml", "part def P; metaclass M specializes P;", "type")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "kind mismatch") {
		t.Errorf("got %q", diags[0].Message)
	}
}

func TestTypeCheckSubsettingUnknownTargetConstrainsNothing(t *testing.T) {
	for _, rel := range []ast.RelationshipKind{ast.RelSubsets, ast.RelRedefines} {
		if got := compatMessage(declKind{}, rel, symbols.SymbolUnknown); got != "" {
			t.Errorf("%s with unknown target = %q, want no diagnostic", rel, got)
		}
	}
}
