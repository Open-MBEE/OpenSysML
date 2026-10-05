package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func relFixtureSymbol(t *testing.T, fixture reflectiveFixture, name string) *symbols.Symbol {
	t.Helper()
	var found *symbols.Symbol
	var walk func(scope *symbols.Scope)
	walk = func(scope *symbols.Scope) {
		for _, sym := range scope.AllMembers() {
			if sym.Name == name && found == nil {
				found = sym
			}
			walk(sym.Scope)
		}
	}
	walk(fixture.root)
	if found == nil {
		t.Fatalf("%s not found in scope", name)
	}
	return found
}

func assertRelFeature(t *testing.T, model *Model, rel *symbols.Symbol, feature string, want ...*symbols.Symbol) {
	t.Helper()
	got, ok := model.ReflectiveElements(rel, feature)
	if !ok {
		t.Errorf("%s on %s is unsupported", feature, relMetaName(model, rel))
		return
	}
	if len(got) != len(want) {
		t.Fatalf("%s on %s = %d elements, want %d", feature, relMetaName(model, rel), len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s on %s: element %d = %s, want %s", feature, relMetaName(model, rel), i,
				model.fqnOf(got[i]), model.fqnOf(want[i]))
		}
	}
}

func assertRelUnsupported(t *testing.T, model *Model, rel *symbols.Symbol, feature string) {
	t.Helper()
	if got, ok := model.ReflectiveElements(rel, feature); ok {
		t.Errorf("%s on %s = %v, want unsupported", feature, relMetaName(model, rel), got)
	}
}

func assertRelBool(t *testing.T, model *Model, rel *symbols.Symbol, feature string, want bool) {
	t.Helper()
	value, ok := model.ReflectiveFeatureValue(rel, feature)
	if !ok {
		t.Fatalf("%s on %s is unsupported", feature, relMetaName(model, rel))
	}
	if value.Kind != symbols.FilterValueBool || value.Bool != want {
		t.Errorf("%s on %s = %v, want %t", feature, relMetaName(model, rel), value.Bool, want)
	}
}

func relMetaName(model *Model, rel *symbols.Symbol) string {
	if meta := model.MetaclassOf(rel); meta != nil {
		return meta.Name
	}
	return "<none>"
}

func assertRelMetaclass(t *testing.T, model *Model, rel *symbols.Symbol, want string) {
	t.Helper()
	if got := relMetaName(model, rel); got != want {
		t.Errorf("metaclass = %s, want %s", got, want)
	}
}

func TestImplicitSubclassification(t *testing.T) {
	src := `package P {
    part def B;
    part def A :> B;
}`
	for _, fixture := range reflectiveFixtures(t, "implicit-subclassification.sysml", source.KindSysML, src) {
		model := fixture.model
		a := relFixtureSymbol(t, fixture, "A")
		b := relFixtureSymbol(t, fixture, "B")
		rels := model.ImplicitRelationships(a)
		if len(rels) != 1 {
			t.Fatalf("ImplicitRelationships(A) = %d, want 1", len(rels))
		}
		rel := rels[0]
		assertRelMetaclass(t, model, rel, "Subclassification")
		assertRelFeature(t, model, rel, "subclassifier", a)
		assertRelFeature(t, model, rel, "superclassifier", b)
		assertRelFeature(t, model, rel, "specific", a)
		assertRelFeature(t, model, rel, "general", b)
		assertRelFeature(t, model, rel, "source", a)
		assertRelFeature(t, model, rel, "target", b)
		assertRelFeature(t, model, rel, "relatedElement", a, b)
		assertRelFeature(t, model, rel, "owningRelatedElement", a)
		assertRelFeature(t, model, rel, "ownedRelatedElement")
		assertRelFeature(t, model, rel, "owningClassifier", a)
		assertRelFeature(t, model, rel, "owningType", a)
		assertRelFeature(t, model, rel, "owner", a)
		assertRelBool(t, model, rel, "isImplied", false)
		assertRelUnsupported(t, model, rel, "subsettingFeature")
		assertRelFeature(t, model, a, "ownedSubclassification", rel)
		assertRelFeature(t, model, a, "ownedSpecialization", rel)
	}
}

func TestImplicitSpecialization(t *testing.T) {
	src := `class U;
type T specializes U;`
	for _, fixture := range reflectiveFixtures(t, "implicit-specialization.kerml", source.KindKerML, src) {
		model := fixture.model
		typ := relFixtureSymbol(t, fixture, "T")
		u := relFixtureSymbol(t, fixture, "U")
		rels := model.ImplicitRelationships(typ)
		if len(rels) != 1 {
			t.Fatalf("ImplicitRelationships(T) = %d, want 1", len(rels))
		}
		rel := rels[0]
		assertRelMetaclass(t, model, rel, "Specialization")
		assertRelFeature(t, model, rel, "specific", typ)
		assertRelFeature(t, model, rel, "general", u)
		assertRelUnsupported(t, model, rel, "subclassifier")
		assertRelUnsupported(t, model, rel, "subsettingFeature")
		assertRelFeature(t, model, typ, "ownedSpecialization", rel)
		assertRelUnsupported(t, model, typ, "ownedSubclassification")
		assertRelUnsupported(t, model, typ, "ownedSubsetting")
	}
}

func TestImplicitFeatureTyping(t *testing.T) {
	src := `package P {
    part def T;
    part p : T;
}`
	for _, fixture := range reflectiveFixtures(t, "implicit-typing.sysml", source.KindSysML, src) {
		model := fixture.model
		p := relFixtureSymbol(t, fixture, "p")
		typ := relFixtureSymbol(t, fixture, "T")
		rels := model.ImplicitRelationships(p)
		if len(rels) != 1 {
			t.Fatalf("ImplicitRelationships(p) = %d, want 1", len(rels))
		}
		rel := rels[0]
		assertRelMetaclass(t, model, rel, "FeatureTyping")
		assertRelFeature(t, model, rel, "typedFeature", p)
		assertRelFeature(t, model, rel, "type", typ)
		assertRelFeature(t, model, rel, "specific", p)
		assertRelFeature(t, model, rel, "general", typ)
		assertRelFeature(t, model, rel, "owningFeature", p)
		assertRelFeature(t, model, p, "ownedTyping", rel)
		assertRelFeature(t, model, p, "ownedSpecialization", rel)
	}
}

func TestImplicitConjugatedPortTyping(t *testing.T) {
	src := `package P {
    port def Pd;
    part def D {
        port p : ~Pd;
    }
}`
	for _, fixture := range reflectiveFixtures(t, "implicit-conjport.sysml", source.KindSysML, src) {
		model := fixture.model
		p := relFixtureSymbol(t, fixture, "p")
		pd := relFixtureSymbol(t, fixture, "Pd")
		rels := model.ImplicitRelationships(p)
		if len(rels) != 1 {
			t.Fatalf("ImplicitRelationships(p) = %d, want 1", len(rels))
		}
		rel := rels[0]
		assertRelMetaclass(t, model, rel, "ConjugatedPortTyping")
		assertRelFeature(t, model, rel, "portDefinition", pd)
		assertRelFeature(t, model, rel, "typedFeature", p)
		assertRelFeature(t, model, rel, "specific", p)
		assertRelUnsupported(t, model, rel, "general")
		assertRelUnsupported(t, model, rel, "type")
		assertRelUnsupported(t, model, rel, "target")
		assertRelUnsupported(t, model, rel, "relatedElement")
		assertRelUnsupported(t, model, rel, "conjugatedPortDefinition")
	}
}

func TestImplicitSubsettingAndRedefinition(t *testing.T) {
	src := `package P {
    part def Base {
        part c;
    }
    part def D :> Base {
        part a;
        part b subsets a;
        part d :>> c;
    }
}`
	for _, fixture := range reflectiveFixtures(t, "implicit-subsetting.sysml", source.KindSysML, src) {
		model := fixture.model
		a := relFixtureSymbol(t, fixture, "a")
		b := relFixtureSymbol(t, fixture, "b")
		c := relFixtureSymbol(t, fixture, "c")
		d := relFixtureSymbol(t, fixture, "d")
		relsB := model.ImplicitRelationships(b)
		if len(relsB) != 1 {
			t.Fatalf("ImplicitRelationships(b) = %d, want 1", len(relsB))
		}
		sub := relsB[0]
		assertRelMetaclass(t, model, sub, "Subsetting")
		assertRelFeature(t, model, sub, "subsettingFeature", b)
		assertRelFeature(t, model, sub, "subsettedFeature", a)
		assertRelFeature(t, model, sub, "source", b)
		assertRelFeature(t, model, sub, "target", a)
		assertRelUnsupported(t, model, sub, "redefinedFeature")
		assertRelUnsupported(t, model, sub, "referencedFeature")
		assertRelFeature(t, model, b, "ownedSubsetting", sub)
		assertRelFeature(t, model, b, "ownedSpecialization", sub)
		relsD := model.ImplicitRelationships(d)
		if len(relsD) != 1 {
			t.Fatalf("ImplicitRelationships(d) = %d, want 1", len(relsD))
		}
		redef := relsD[0]
		assertRelMetaclass(t, model, redef, "Redefinition")
		assertRelFeature(t, model, redef, "redefiningFeature", d)
		assertRelFeature(t, model, redef, "redefinedFeature", c)
		assertRelFeature(t, model, redef, "subsettingFeature", d)
		assertRelFeature(t, model, redef, "subsettedFeature", c)
		assertRelFeature(t, model, d, "ownedRedefinition", redef)
		assertRelFeature(t, model, d, "ownedSubsetting", redef)
	}
}

func TestImplicitReferenceSubsetting(t *testing.T) {
	src := `package P {
    part def D {
        part f;
        part e references f;
    }
}`
	for _, fixture := range reflectiveFixtures(t, "implicit-refsubsetting.sysml", source.KindSysML, src) {
		model := fixture.model
		e := relFixtureSymbol(t, fixture, "e")
		f := relFixtureSymbol(t, fixture, "f")
		rels := model.ImplicitRelationships(e)
		if len(rels) != 1 {
			t.Fatalf("ImplicitRelationships(e) = %d, want 1", len(rels))
		}
		rel := rels[0]
		assertRelMetaclass(t, model, rel, "ReferenceSubsetting")
		assertRelFeature(t, model, rel, "referencingFeature", e)
		assertRelFeature(t, model, rel, "referencedFeature", f)
		assertRelFeature(t, model, rel, "subsettingFeature", e)
		assertRelFeature(t, model, rel, "subsettedFeature", f)
		assertRelFeature(t, model, e, "ownedReferenceSubsetting", rel)
		assertRelFeature(t, model, e, "ownedSubsetting", rel)
	}
}

func TestImplicitCrossSubsetting(t *testing.T) {
	src := `class C {
    feature f;
    feature g crosses f;
}`
	for _, fixture := range reflectiveFixtures(t, "implicit-crosssubsetting.kerml", source.KindKerML, src) {
		model := fixture.model
		g := relFixtureSymbol(t, fixture, "g")
		f := relFixtureSymbol(t, fixture, "f")
		rels := model.ImplicitRelationships(g)
		if len(rels) != 1 {
			t.Fatalf("ImplicitRelationships(g) = %d, want 1", len(rels))
		}
		rel := rels[0]
		assertRelMetaclass(t, model, rel, "CrossSubsetting")
		assertRelFeature(t, model, rel, "crossingFeature", g)
		assertRelFeature(t, model, rel, "crossedFeature", f)
		assertRelFeature(t, model, rel, "subsettingFeature", g)
		assertRelFeature(t, model, rel, "subsettedFeature", f)
		assertRelFeature(t, model, g, "ownedCrossSubsetting", rel)
	}
}

func TestImplicitConjugation(t *testing.T) {
	src := `class A;
class B conjugates A;`
	for _, fixture := range reflectiveFixtures(t, "implicit-conjugation.kerml", source.KindKerML, src) {
		model := fixture.model
		a := relFixtureSymbol(t, fixture, "A")
		b := relFixtureSymbol(t, fixture, "B")
		rels := model.ImplicitRelationships(b)
		if len(rels) != 1 {
			t.Fatalf("ImplicitRelationships(B) = %d, want 1", len(rels))
		}
		rel := rels[0]
		assertRelMetaclass(t, model, rel, "Conjugation")
		assertRelFeature(t, model, rel, "conjugatedType", b)
		assertRelFeature(t, model, rel, "originalType", a)
		assertRelFeature(t, model, rel, "source", b)
		assertRelFeature(t, model, rel, "target", a)
		assertRelFeature(t, model, rel, "owningType", b)
		assertRelUnsupported(t, model, rel, "specific")
		assertRelUnsupported(t, model, rel, "general")
		assertRelFeature(t, model, b, "ownedConjugator", rel)
		assertRelFeature(t, model, b, "ownedSpecialization")
		assertRelBool(t, model, b, "isConjugated", true)
		assertRelBool(t, model, a, "isConjugated", false)
	}
}

func TestImplicitMultiplicitySubsets(t *testing.T) {
	src := `package P {
    part def D {
        part x[1];
        part y[2] subsets x;
    }
}`
	for _, fixture := range reflectiveFixtures(t, "implicit-multsubsets.sysml", source.KindSysML, src) {
		model := fixture.model
		y := relFixtureSymbol(t, fixture, "y")
		x := relFixtureSymbol(t, fixture, "x")
		rels := model.ImplicitRelationships(y)
		var sub *symbols.Symbol
		for _, rel := range rels {
			if relMetaName(model, rel) == "Subsetting" {
				sub = rel
			}
		}
		if sub == nil {
			t.Fatalf("no Subsetting relationship on y; %d rels", len(rels))
		}
		assertRelFeature(t, model, sub, "subsettingFeature", y)
		assertRelFeature(t, model, sub, "subsettedFeature", x)
	}
}

func TestKeywordSpecializationDerivations(t *testing.T) {
	src := `package P {
    classifier B;
    classifier A;
    specialization Gen subtype A specializes B;
}`
	for _, fixture := range reflectiveFixtures(t, "keyword-specialization.kerml", source.KindKerML, src) {
		model := fixture.model
		gen := relFixtureSymbol(t, fixture, "Gen")
		a := relFixtureSymbol(t, fixture, "A")
		b := relFixtureSymbol(t, fixture, "B")
		assertRelMetaclass(t, model, gen, "Specialization")
		assertRelFeature(t, model, gen, "specific", a)
		assertRelFeature(t, model, gen, "general", b)
		assertRelFeature(t, model, gen, "source", a)
		assertRelFeature(t, model, gen, "target", b)
		assertRelFeature(t, model, gen, "relatedElement", a, b)
		assertRelFeature(t, model, gen, "owningRelatedElement")
		assertRelBool(t, model, gen, "isImplied", false)
	}
}

func TestImplicitRelationshipOrdering(t *testing.T) {
	src := `package P {
    part def T;
    part def D {
        part a;
        part c;
        part p : T :> a;
        part b :>> c;
    }
}`
	for _, fixture := range reflectiveFixtures(t, "implicit-ordering.sysml", source.KindSysML, src) {
		model := fixture.model
		p := relFixtureSymbol(t, fixture, "p")
		rels := model.ImplicitRelationships(p)
		if len(rels) != 2 {
			t.Fatalf("ImplicitRelationships(p) = %d, want 2", len(rels))
		}
		assertRelMetaclass(t, model, rels[0], "FeatureTyping")
		assertRelMetaclass(t, model, rels[1], "Subsetting")
		assertRelFeature(t, model, p, "ownedSpecialization", rels[0], rels[1])
		assertRelFeature(t, model, p, "ownedTyping", rels[0])
		assertRelFeature(t, model, p, "ownedSubsetting", rels[1])
		// identity stability
		again := model.ImplicitRelationships(p)
		for i := range rels {
			if again[i] != rels[i] {
				t.Errorf("ImplicitRelationships(p)[%d] pointer changed", i)
			}
			if !symbols.SameElement(rels[i], again[i]) {
				t.Errorf("SameElement failed for relationship %d", i)
			}
		}
		if symbols.SameElement(rels[0], rels[1]) {
			t.Error("SameElement(rels[0], rels[1]) = true, want false")
		}
		if symbols.SameElement(rels[0], p) {
			t.Error("SameElement(rel, owner) = true, want false")
		}
		if symbols.KeyOf(rels[0]) == symbols.KeyOf(rels[1]) {
			t.Error("KeyOf collides for distinct relationships")
		}
		if symbols.KeyOf(rels[0]) == symbols.KeyOf(p) {
			t.Error("KeyOf collides with owner")
		}
	}
}

func TestImplicitChainTargetUnsupported(t *testing.T) {
	src := `package P {
    part def D {
        part a {
            part x;
        }
        part b :> a.x;
    }
}`
	for _, fixture := range reflectiveFixtures(t, "implicit-chain.sysml", source.KindSysML, src) {
		model := fixture.model
		b := relFixtureSymbol(t, fixture, "b")
		rels := model.ImplicitRelationships(b)
		if len(rels) != 1 {
			t.Fatalf("ImplicitRelationships(b) = %d, want 1", len(rels))
		}
		rel := rels[0]
		assertRelFeature(t, model, rel, "subsettingFeature", b)
		assertRelUnsupported(t, model, rel, "subsettedFeature")
		assertRelUnsupported(t, model, rel, "target")
		assertRelUnsupported(t, model, rel, "general")
		assertRelUnsupported(t, model, rel, "relatedElement")
	}
}

func TestImplicitRelationshipNegatives(t *testing.T) {
	src := `package P {
    part def D {
        part a;
        part b subsets a;
    }
}`
	for _, fixture := range reflectiveFixtures(t, "implicit-negatives.sysml", source.KindSysML, src) {
		model := fixture.model
		b := relFixtureSymbol(t, fixture, "b")
		d := relFixtureSymbol(t, fixture, "D")
		rel := model.ImplicitRelationships(b)[0]
		assertRelUnsupported(t, model, rel, "redefinedFeature")
		assertRelUnsupported(t, model, rel, "referencingFeature")
		assertRelUnsupported(t, model, d, "ownedSubsetting")
		assertRelUnsupported(t, model, b, "ownedRelationship")
		assertRelUnsupported(t, model, b, "ownedSubclassification")
	}
}
