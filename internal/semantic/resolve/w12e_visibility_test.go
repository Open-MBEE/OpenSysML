package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A connector end's participant is a feature of the type featuring the
// connector, so the connector's own features are not participants
// (KerML 8.3.4.5).
func TestConnectorEndDoesNotReferenceAFeatureOfTheConnector(t *testing.T) {
	r := resolveVisibilityDoc(t, `package test {
		feature x;
		connector c {
			feature f;
			end a ::> x;
			end b ::> f;
		}
	}`)
	wantUnresolved(t, r, "f")
	wantNoUnresolved(t, r, "x")
}

// `connector eng to t;` names no connector (KerML.xtext:836): `eng` stays the
// one feature, and a named end's name is not visible to the featuring type.
func TestKerMLBinaryConnectorFirstEndIsAnEnd(t *testing.T) {
	r := resolveVisibilityDoc(t, `package test {
		class T {
			feature eng; feature t; feature u;
			connector eng to t;
			connector a ::> eng to t;
			connector [0..1] eng to [1..*] u;
			feature viaEng :> eng;
			feature viaEnd :> a;
		}
	}`)
	wantNoUnresolved(t, r, "eng", "t", "u")
	wantUnresolved(t, r, "a")
	eng := r.Index().LookupQualified("test::T::eng")
	if len(eng) != 1 {
		t.Fatalf("T declares %d symbols named eng, want 1", len(eng))
	}
	if got := r.Index().LookupQualified("test::T::a"); len(got) != 0 {
		t.Errorf("the end name a is a member of T: %v", got)
	}
	via := r.Index().LookupQualified("test::T::viaEng")[0].Decl.(*ast.Usage)
	if len(via.Relationships) != 1 {
		t.Fatalf("viaEng declares %d relationships, want its one subsetting", len(via.Relationships))
	}
	for _, rel := range via.Relationships {
		qn := rel.Target.(*ast.QualifiedName)
		if got, ok := r.PartSymbol(qn, 0); !ok || got != eng[0] {
			t.Errorf("viaEng :> eng resolved to %v, want the feature eng", got)
		}
	}
}

// An end outside a connector relates nothing, so its reference subsetting sees
// the enclosing declaration's features as any other reference does.
func TestEndOutsideAConnectorReferencesItsOwnersFeature(t *testing.T) {
	r := resolveVisibilityDoc(t, `package test {
		feature Pattern {
			feature source;
			end s ::> source;
		}
	}`)
	wantNoUnresolved(t, r, "source")
}

// A feature with no declared name takes the name of the feature it redefines,
// so it binds no name when that redefinition resolves to nothing, and a later
// redefinition of the same name does not find it (KerML 7.3.4.5).
func TestUnnamedRedefinitionOfAnInvisibleFeatureBindsNoName(t *testing.T) {
	r := resolveVisibilityDoc(t, `package test {
		feature x {
			feature a;
			private feature c;
		}
		feature y subsets x {
			feature redefines a;
			feature redefines c;
		}
		feature redefines x.c;
	}`)
	wantUnresolved(t, r, "c")
	wantNoUnresolved(t, r, "a")
}

// A binding's named end reference-subsets a feature of the binding's owner
// (`binding of e1 ::> a = b;`, KerML.xtext:854): `a` resolves there, the end
// name is the binding's member alone, and a named binding's end is reached through it.
func TestBindingConnectorEndNamesAreEndsAndTheirTargetsResolve(t *testing.T) {
	r := resolveVisibilityDoc(t, `package test {
		class Base { feature inherited; }
		class T :> Base {
			feature a; feature b;
			binding a = b;
			binding of e1 ::> a = e2 references b;
			binding bb of e3 ::> inherited = [1] e4 ::> b;
			feature viaA :> a;
			feature viaEnd :> e1;
			feature viaBinding :> bb.e3;
		}
	}`)
	wantNoUnresolved(t, r, "a", "b", "inherited", "bb.e3")
	wantUnresolved(t, r, "e1")
	a := r.Index().LookupQualified("test::T::a")
	if len(a) != 1 {
		t.Fatalf("T declares %d symbols named a, want 1", len(a))
	}
	for _, name := range []string{"e1", "e2", "e3", "e4"} {
		if got := r.Index().LookupQualified("test::T::" + name); len(got) != 0 {
			t.Errorf("the end name %s is a member of T: %v", name, got)
		}
	}
	if got := r.Index().LookupQualified("test::T::bb::e3"); len(got) != 1 {
		t.Errorf("bb declares %d ends named e3, want 1", len(got))
	}
	bindings := 0
	for _, sym := range r.Index().LookupQualified("test::T::bb") {
		u, ok := sym.Decl.(*ast.Usage)
		if !ok {
			continue
		}
		bindings++
		if len(u.ConnectorEnds) != 2 {
			t.Fatalf("bb carries %d ends, want 2", len(u.ConnectorEnds))
		}
		qn, ok := u.ConnectorEnds[0].AttachedTarget().(*ast.QualifiedName)
		if !ok {
			t.Fatalf("bb's first end attaches %T, want a qualified name", u.ConnectorEnds[0].AttachedTarget())
		}
		if got, ok := r.PartSymbol(qn, 0); !ok || got.Name != "inherited" {
			t.Errorf("e3 ::> inherited resolved to %v, want Base's feature inherited", got)
		}
	}
	if bindings != 1 {
		t.Errorf("T declares %d bindings named bb, want 1", bindings)
	}
}
