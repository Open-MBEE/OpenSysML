package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestW7BRelationshipMetaclassNamesAreDeclared guards the relationship kind →
// metaclass mapping against a name no KerML metaclass package declares.
func TestW7BRelationshipMetaclassNamesAreDeclared(t *testing.T) {
	m := stdlibModel(t)
	names := []string{"Conjugation"}
	for _, name := range relationshipMetaclassNames {
		names = append(names, name)
	}
	for _, name := range names {
		if m.kermlMetaclass(name) == nil {
			t.Errorf("relationship metaclass %s is not declared by any KerML metaclass package", name)
		}
	}
}

// A relationship written keyword-first classifies as its own metaclass, not as
// the usage the superseded representation made of it (KerML §7.2, §8.2.4).
func TestW7BKeywordFirstRelationshipClassifiesAsItsMetaclass(t *testing.T) {
	const src = `
		class A;
		class B;
		specialization Gen subclassifier A specializes B;
		conjugation Conj conjugate A conjugates B;
		disjoining Dis disjoint A from B;
	`
	m, root := stdlibModelWithDoc(t, "w7b_metaclass.kerml", src)
	for _, tc := range []struct{ name, metaclass string }{
		{"Gen", "Specialization"},
		{"Conj", "Conjugation"},
		{"Dis", "Disjoining"},
	} {
		elem := sym(t, root, tc.name)
		got := m.metaclassOf(elem)
		if got == nil {
			t.Fatalf("%s has no metaclass, want %s", tc.name, tc.metaclass)
		}
		if got.Name != tc.metaclass {
			t.Errorf("%s classifies as %s, want %s", tc.name, got.Name, tc.metaclass)
		}
		if _, ok := elem.Decl.(*ast.RelationshipMember); !ok {
			t.Errorf("%s is declared by %T, want a relationship member", tc.name, elem.Decl)
		}
		if !m.metaclassConforms(elem, "KerML::Core::"+tc.metaclass) {
			t.Errorf("%s should conform to the %s metaclass", tc.name, tc.metaclass)
		}
	}
}

// stdlibModelWithDoc resolves src against the standard library, so that a
// declaration can be classified by a library-declared metaclass.
func stdlibModelWithDoc(t *testing.T, name, src string) (*Model, *symbols.Scope) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := stdlibIndex(t)
	idx.AddDocument(name, root)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)
	return m, idx.DocumentRoot(name)
}

// A named multiplicity classifies as the KerML Multiplicity metaclass in either
// language — a range as MultiplicityRange, which specializes it — so a filter
// for `Multiplicity` keeps it.
func TestNamedMultiplicityClassifiesAsMultiplicity(t *testing.T) {
	for _, tc := range []struct{ file, src string }{
		{"multiplicity.kerml", "multiplicity one [1];\nmultiplicity some subsets one;\n"},
		{"multiplicity.sysml", "package P {\n\tmultiplicity one [1];\n\tmultiplicity some subsets one;\n}\n"},
	} {
		m, root := stdlibModelWithDoc(t, tc.file, tc.src)
		if strings.HasSuffix(tc.file, ".sysml") {
			root = sym(t, root, "P").Scope
		}
		for _, tm := range []struct{ name, metaclass string }{
			{"one", "MultiplicityRange"},
			{"some", "Multiplicity"},
		} {
			elem := sym(t, root, tm.name)
			got := m.metaclassOf(elem)
			if got == nil {
				t.Fatalf("%s: %s has no metaclass, want %s", tc.file, tm.name, tm.metaclass)
			}
			if got.Name != tm.metaclass {
				t.Errorf("%s: %s classifies as %s, want %s", tc.file, tm.name, got.Name, tm.metaclass)
			}
			if !m.metaclassConforms(elem, "KerML::Kernel::Multiplicity") {
				t.Errorf("%s: %s should conform to the Multiplicity metaclass", tc.file, tm.name)
			}
			if !m.metaclassConforms(elem, "KerML::Core::Feature") {
				t.Errorf("%s: %s should conform to Feature, which Multiplicity specializes", tc.file, tm.name)
			}
			if m.metaclassConforms(elem, "KerML::Core::Classifier") {
				t.Errorf("%s: %s should not conform to Classifier", tc.file, tm.name)
			}
		}
	}
}
