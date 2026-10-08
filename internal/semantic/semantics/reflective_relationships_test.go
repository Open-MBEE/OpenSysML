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

// A chain target (`:> a.x`) is reflected as the chaining feature the chain
// denotes as a whole (KerML 8.3.3.3.9): a Feature the relationship targets and
// owns, whose chainingFeature are the chain's features in order, the same for a
// declaration and for its record.
func TestImplicitChainTargetIsChainingFeature(t *testing.T) {
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
		a := relFixtureSymbol(t, fixture, "a")
		x := relFixtureSymbol(t, fixture, "x")
		b := relFixtureSymbol(t, fixture, "b")
		rels := model.ImplicitRelationships(b)
		if len(rels) != 1 {
			t.Fatalf("ImplicitRelationships(b) = %d, want 1", len(rels))
		}
		rel := rels[0]
		assertRelFeature(t, model, rel, "subsettingFeature", b)
		targets, ok := model.ReflectiveElements(rel, "target")
		if !ok || len(targets) != 1 {
			t.Fatalf("target on %s = %v, %t; want one chaining feature", relMetaName(model, rel), targets, ok)
		}
		chain := targets[0]
		if chain.Chain == nil || chain.Chain.Relationship != rel {
			t.Fatalf("target of %s is %s, want the feature its chain denotes", relMetaName(model, rel), model.fqnOf(chain))
		}
		if got := model.fqnOf(model.MetaclassOf(chain)); got != "KerML::Core::Feature" {
			t.Errorf("chain target metaclass = %s, want KerML::Core::Feature", got)
		}
		assertRelFeature(t, model, rel, "subsettedFeature", chain)
		assertRelFeature(t, model, rel, "general", chain)
		assertRelFeature(t, model, rel, "relatedElement", b, chain)
		assertRelFeature(t, model, rel, "ownedRelatedElement", chain)
		assertRelFeature(t, model, chain, "chainingFeature", a, x)
		assertRelFeature(t, model, chain, "owner", b)
		assertRelFeature(t, model, b, "ownedElement", chain)
		if again, _ := model.ReflectiveElements(rel, "target"); len(again) != 1 || again[0] != chain {
			t.Errorf("target of %s changed between reads", relMetaName(model, rel))
		}
		for _, other := range []*symbols.Symbol{rel, b} {
			if symbols.KeyOf(chain) == symbols.KeyOf(other) || symbols.SameElement(chain, other) {
				t.Errorf("KeyOf(chain target) collides with %s", model.fqnOf(other))
			}
		}
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

// `include use case uc : UC` echoes its typing target as an includes edge the
// abstract syntax does not declare, so only the shorthand yields a
// ReferenceSubsetting object.
func TestImplicitIncludeUseCaseEcho(t *testing.T) {
	src := `package P {
    use case def UC1;
    use case uc2;
    use case def D {
        include use case uc1 : UC1;
        include uc2;
    }
}`
	for _, fixture := range reflectiveFixtures(t, "include-echo.sysml", source.KindSysML, src) {
		model := fixture.model
		uc1 := relFixtureSymbol(t, fixture, "uc1")
		rels := model.ImplicitRelationships(uc1)
		if len(rels) != 1 {
			t.Fatalf("ImplicitRelationships(uc1) = %d, want 1", len(rels))
		}
		assertRelMetaclass(t, model, rels[0], "FeatureTyping")
		// The include shorthand still reflects a ReferenceSubsetting.
		var ref *symbols.Symbol
		var walk func(scope *symbols.Scope)
		walk = func(scope *symbols.Scope) {
			for _, sym := range scope.AllMembers() {
				for _, rel := range model.ImplicitRelationships(sym) {
					if relMetaName(model, rel) == "ReferenceSubsetting" {
						ref = rel
					}
				}
				walk(sym.Scope)
			}
		}
		walk(fixture.root)
		if ref == nil {
			t.Fatal("no ReferenceSubsetting reflected for the include shorthand")
		}
		if elems, ok := model.ReflectiveElements(ref, "referencingFeature"); !ok || len(elems) != 1 {
			t.Errorf("referencingFeature = %v (supported %t), want the include usage", elems, ok)
		}
		if elems, ok := model.ReflectiveElements(ref, "referencedFeature"); !ok || len(elems) != 1 ||
			elems[0] != relFixtureSymbol(t, fixture, "uc2") {
			t.Errorf("referencedFeature = %v (supported %t), want uc2", elems, ok)
		}
	}
}

// A keyword relationship member with an unresolved or chained end leaves the
// features on that side underived, like a written edge.
func TestKeywordRelationshipMemberUnresolvedEnds(t *testing.T) {
	src := `package P {
    classifier B;
    specialization Gen subtype Missing specializes B;
    specialization Chain subtype B specializes B.self;
}`
	for _, fixture := range reflectiveFixtures(t, "keyword-unresolved.kerml", source.KindKerML, src) {
		model := fixture.model
		gen := relFixtureSymbol(t, fixture, "Gen")
		assertRelUnsupported(t, model, gen, "specific")
		assertRelUnsupported(t, model, gen, "source")
		assertRelFeature(t, model, gen, "general", relFixtureSymbol(t, fixture, "B"))
		chain := relFixtureSymbol(t, fixture, "Chain")
		assertRelUnsupported(t, model, chain, "general")
		assertRelUnsupported(t, model, chain, "target")
		assertRelUnsupported(t, model, chain, "relatedElement")
	}
}

// An extended definition or usage classifies as Definition/Usage whatever its
// keyword names it.
func TestExtendedDefinitionUsageMetaclass(t *testing.T) {
	src := `package P {
    metadata def service;
    #service def APISService;
}`
	for _, fixture := range reflectiveFixtures(t, "extended-def.sysml", source.KindSysML, src) {
		model := fixture.model
		svc := relFixtureSymbol(t, fixture, "APISService")
		assertRelMetaclass(t, model, svc, "Definition")
	}
	kermlSrc := `class C;
class D {
    class x;
}`
	for _, fixture := range reflectiveFixtures(t, "extended-usage.kerml", source.KindKerML, kermlSrc) {
		model := fixture.model
		x := relFixtureSymbol(t, fixture, "x")
		if meta := model.MetaclassOf(x); meta == nil {
			t.Error("class usage has no metaclass")
		}
	}
	sysmlUsage := `package P {
    class x;
}`
	for _, fixture := range reflectiveFixtures(t, "extended-usage2.sysml", source.KindSysML, sysmlUsage) {
		model := fixture.model
		x := relFixtureSymbol(t, fixture, "x")
		// KerML-notation declarations in a SysML document are not extended
		// usages: `class x` keeps no metaclass, as before extended
		// definitions classified.
		if meta := model.MetaclassOf(x); meta != nil {
			t.Errorf("class x metaclass = %s, want none", model.fqnOf(meta))
		}
	}
	svcUsage := `package P {
    metadata def service;
    #service s;
}`
	for _, fixture := range reflectiveFixtures(t, "extended-usage3.sysml", source.KindSysML, svcUsage) {
		model := fixture.model
		s := relFixtureSymbol(t, fixture, "s")
		assertRelMetaclass(t, model, s, "ReferenceUsage")
	}
}

// A recorded owner lists the same relationship objects as its AST self,
// including a multiplicity's `subsets` and the include echo the AST skips.
func TestImplicitRelationshipsASTRecordedParity(t *testing.T) {
	check := func(t *testing.T, fixtures []reflectiveFixture, owners []string) {
		t.Helper()
		astModel, recModel := fixtures[0].model, fixtures[1].model
		for _, path := range owners {
			astSym := nestedSym(t, fixtures[0].root, path)
			recSym := nestedSym(t, fixtures[1].root, path)
			astRels := astModel.ImplicitRelationships(astSym)
			recRels := recModel.ImplicitRelationships(recSym)
			if len(astRels) != len(recRels) {
				t.Fatalf("%s: %d AST objects, %d recorded", path, len(astRels), len(recRels))
			}
			for i := range astRels {
				ar, rr := astRels[i], recRels[i]
				if astModel.implicitMetaclassName(ar) != recModel.implicitMetaclassName(rr) {
					t.Errorf("%s rel %d: metaclass %s vs recorded %s", path, i,
						astModel.implicitMetaclassName(ar), recModel.implicitMetaclassName(rr))
				}
				if ar.Implicit.Ordinal != rr.Implicit.Ordinal {
					t.Errorf("%s rel %d: ordinal %d vs recorded %d", path, i, ar.Implicit.Ordinal, rr.Implicit.Ordinal)
				}
				if symbols.KeyOf(ar) != symbols.KeyOf(rr) {
					t.Errorf("%s rel %d: KeyOf %s vs recorded %s", path, i, symbols.KeyOf(ar), symbols.KeyOf(rr))
				}
				_, astTgt, astOK := astModel.implicitRelationshipEnds(ar)
				_, recTgt, recOK := recModel.implicitRelationshipEnds(rr)
				if astOK != recOK {
					t.Errorf("%s rel %d: target resolved %t vs recorded %t", path, i, astOK, recOK)
					continue
				}
				if astOK && astModel.fqnOf(astTgt) != recModel.fqnOf(recTgt) {
					t.Errorf("%s rel %d: target %s vs recorded %s", path, i,
						astModel.fqnOf(astTgt), recModel.fqnOf(recTgt))
				}
			}
		}
	}
	sysmlSrc := `package P {
    part def A;
    port def Pd;
    use case def UC1;
    part f { part g; }
    part a : A;
    part b :> A;
    part c :>> a;
    part d ::> a;
    port p : ~Pd;
    part e :> f.g;
    multiplicity one [1];
    multiplicity some subsets one;
    use case uc2 : UC1;
    use case uc {
        include use case uc1 : UC1;
        include uc2;
    }
}`
	check(t, reflectiveFixtures(t, "parity.sysml", source.KindSysML, sysmlSrc),
		[]string{"P::a", "P::b", "P::c", "P::d", "P::p", "P::e", "P::some", "P::uc"})
	kermlSrc := `class CA;
class CB conjugates CA;
class KC { feature kf; }
class KD specializes KC { feature x crosses kf crosses kf; }`
	check(t, reflectiveFixtures(t, "parity.kerml", source.KindKerML, kermlSrc),
		[]string{"CB", "KD::x"})
}
