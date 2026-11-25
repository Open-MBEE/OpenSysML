package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A keyword-first Disjoining, FeatureInverting or TypeFeaturing answers the ends its metaclass
// declares and the Relationship ends they redefine, and has no owning related element.
func TestKeywordRelationshipMemberEnds(t *testing.T) {
	src := `package K {
	class A;
	class B;
	feature f : A;
	feature g : A;
	disjoining dn disjoint A from B;
	inverting vn inverse f of g;
	featuring fn of f by A;
}`
	cases := []struct {
		member, metaclass      string
		sourceEnd, targetEnd   string
		source, target, owning string
	}{
		{"dn", "Disjoining", "typeDisjoined", "disjoiningType", "A", "B", "owningType"},
		{"vn", "FeatureInverting", "featureInverted", "invertingFeature", "f", "g", "owningFeature"},
		{"fn", "TypeFeaturing", "featureOfType", "featuringType", "f", "A", "owningFeatureOfType"},
	}
	for _, fixture := range reflectiveFixtures(t, "keyword-ends.kerml", source.KindKerML, src) {
		model := fixture.model
		for _, c := range cases {
			rel := relFixtureSymbol(t, fixture, c.member)
			src, tgt := relFixtureSymbol(t, fixture, c.source), relFixtureSymbol(t, fixture, c.target)
			assertRelMetaclass(t, model, rel, c.metaclass)
			assertRelFeature(t, model, rel, c.sourceEnd, src)
			assertRelFeature(t, model, rel, c.targetEnd, tgt)
			assertRelFeature(t, model, rel, "source", src)
			assertRelFeature(t, model, rel, "target", tgt)
			assertRelFeature(t, model, rel, "relatedElement", src, tgt)
			assertRelFeature(t, model, rel, c.owning)
			assertRelFeature(t, model, rel, "owningRelatedElement")
			assertRelBool(t, model, rel, "isImplied", false)
		}
	}
}

// A keyword-first relationship whose end does not resolve leaves that end and what it redefines underived.
func TestKeywordRelationshipMemberEndsUnresolved(t *testing.T) {
	src := `package K {
	class A { feature b : A; }
	feature f : A;
	disjoining dn disjoint A from Missing;
	inverting vn inverse Missing of f;
	inverting cv inverse f.b of f;
}`
	for _, fixture := range reflectiveFixtures(t, "keyword-ends-unresolved.kerml", source.KindKerML, src) {
		model := fixture.model
		dn := relFixtureSymbol(t, fixture, "dn")
		assertRelFeature(t, model, dn, "typeDisjoined", relFixtureSymbol(t, fixture, "A"))
		assertRelUnsupported(t, model, dn, "disjoiningType")
		assertRelUnsupported(t, model, dn, "target")
		assertRelUnsupported(t, model, dn, "relatedElement")
		vn := relFixtureSymbol(t, fixture, "vn")
		assertRelUnsupported(t, model, vn, "featureInverted")
		assertRelUnsupported(t, model, vn, "source")
		assertRelFeature(t, model, vn, "invertingFeature", relFixtureSymbol(t, fixture, "f"))
		cv := relFixtureSymbol(t, fixture, "cv")
		assertRelUnsupported(t, model, cv, "featureInverted")
		assertRelUnsupported(t, model, cv, "source")
		assertRelFeature(t, model, cv, "target", relFixtureSymbol(t, fixture, "f"))
	}
}

// A connector's source, target and relatedElement are its sourceFeature, targetFeature and
// relatedFeature, which redefine them (KerML 8.3.4.5.3).
func TestConnectorRelationshipEnds(t *testing.T) {
	check := func(t *testing.T, fixtures []reflectiveFixture, connectors map[string][2]string) {
		t.Helper()
		for _, fixture := range fixtures {
			for connector, ends := range connectors {
				sym := nestedSym(t, fixture.root, connector)
				assertReflectiveElements(t, fixture.model, sym, "source", ends[0])
				assertReflectiveElements(t, fixture.model, sym, "target", ends[1])
				assertReflectiveElements(t, fixture.model, sym, "relatedElement", ends[0], ends[1])
			}
		}
	}
	check(t, reflectiveFixtures(t, "connector-ends.kerml", source.KindKerML, `package K {
	feature f;
	feature g;
	binding bn of f = g;
	succession sn first f then g;
	connector cn from f to g;
}`), map[string][2]string{
		"K::bn": {"K::f", "K::g"},
		"K::sn": {"K::f", "K::g"},
		"K::cn": {"K::f", "K::g"},
	})
	check(t, reflectiveFixtures(t, "connector-ends.sysml", source.KindSysML, `package S {
	part def A;
	part a1 : A;
	part a2 : A;
	binding bn bind a1 = a2;
	action act { action s1; action s2; succession sn first s1 then s2; }
	part ctx { connection cn connect a1 to a2; }
}`), map[string][2]string{
		"S::bn":      {"S::a1", "S::a2"},
		"S::act::sn": {"S::act::s1", "S::act::s2"},
		"S::ctx::cn": {"S::a1", "S::a2"},
	})
}

// A connector whose first end references nothing answers no source, target or relatedElement:
// the resolved end is not read as the source. One whose later end references nothing keeps its source.
func TestConnectorRelationshipEndsUnresolved(t *testing.T) {
	m, root := buildModelWithStdlibNamedKind(t, "connector-unresolved.kerml", source.KindKerML, `package K {
	feature g;
	connector cn from Missing to g;
	connector later from g to Missing;
}`)
	cn := nestedSym(t, root, "K::cn")
	for _, feature := range []string{"source", "target", "relatedElement"} {
		if got, ok := m.ReflectiveElements(cn, feature); ok {
			t.Errorf("K::cn.%s = %v, want underived", feature, fqns(got))
		}
	}
	later := nestedSym(t, root, "K::later")
	for feature, want := range map[string][]string{"source": {"K::g"}, "target": nil, "relatedElement": {"K::g"}} {
		got, ok := m.ReflectiveElements(later, feature)
		if !ok || !slices.Equal(fqns(got), want) {
			t.Errorf("K::later.%s = %v (derived %t), want %v", feature, fqns(got), ok, want)
		}
	}
}

// A dependency's source and target are its clients and suppliers, which redefine them (KerML 8.3.2.2.2).
func TestDependencyRelationshipEnds(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package S {
	part def A;
	part def B;
	part def C;
	dependency dn from A, B to C;
}`)
	dn := nestedSym(t, root, "S::dn")
	assertReflectiveElements(t, m, dn, "source", "S::A", "S::B")
	assertReflectiveElements(t, m, dn, "target", "S::C")
	assertReflectiveElements(t, m, dn, "relatedElement", "S::A", "S::B", "S::C")
	assertReflectiveElements(t, m, dn, "client", "S::A", "S::B")
	assertReflectiveElements(t, m, dn, "supplier", "S::C")
}

// A part or a definition is no relationship: it has no source, target or relatedElement.
func TestRelationshipEndsOfNonRelationships(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package S { part def A; part a : A; }`)
	for _, name := range []string{"S::A", "S::a"} {
		for _, feature := range []string{"source", "target", "relatedElement"} {
			if got, ok := m.ReflectiveElements(nestedSym(t, root, name), feature); ok {
				t.Errorf("%s.%s = %v, want underived", name, feature, fqns(got))
			}
		}
	}
}

// The operator of a collect, select and feature chain expression is the one the specification
// fixes for its metaclass (KerML 8.3.4.8.2, 8.3.4.8.18, 8.3.4.8.4).
func TestFixedExpressionOperators(t *testing.T) {
	m, root := buildModelWithStdlibNamedKind(t, "operators.kerml", source.KindKerML, `package K {
	class A { feature b : A; }
	feature q : A;
	feature chain = q.b;
	feature collect = q.{in p; p};
	feature select = q.?{in p; true};
	feature reference = q;
}`)
	for name, want := range map[string]struct{ metaclass, operator string }{
		"chain":   {"FeatureChainExpression", "."},
		"collect": {"CollectExpression", "collect"},
		"select":  {"SelectExpression", "select"},
	} {
		value := m.ValueExpression(nestedSym(t, root, "K::"+name))
		if value == nil {
			t.Fatalf("K::%s has no value expression", name)
		}
		assertRelMetaclass(t, m, value, want.metaclass)
		got, ok := m.ReflectiveFeatureValue(value, "operator")
		if !ok || got.Kind != symbols.FilterValueString || got.Str != want.operator {
			t.Errorf("operator of K::%s = %v (derived %t), want %q", name, got, ok, want.operator)
		}
	}
	reference := m.ValueExpression(nestedSym(t, root, "K::reference"))
	if got, ok := m.ReflectiveFeatureValue(reference, "operator"); ok {
		t.Errorf("a FeatureReferenceExpression answers operator %v, want underived", got)
	}
}

// The typing a transition trigger writes resolves where the trigger is written, whichever of
// the trigger's parts is asked first and with no document walk before it.
func TestTriggerPayloadTypingResolves(t *testing.T) {
	m, root := buildModelWithStdlibUnwalked(t, `package T {
	item def Sig;
	state def S { state s1; state s2; transition t first s1 accept Sig then s2; }
	state def S2 { state s1; state s2; transition t2 first s1 accept x : Sig then s2; }
}`)
	for _, name := range []string{"T::S::t", "T::S2::t2"} {
		parts := m.transitionPartsOf(nestedSym(t, root, name))
		if parts == nil || parts.accepter == nil || parts.payload == nil {
			t.Fatalf("%s has no trigger accept with a payload", name)
		}
		for _, part := range []*symbols.Symbol{parts.accepter, parts.payload} {
			for _, rel := range m.ImplicitRelationships(part) {
				assertRelMetaclass(t, m, rel, "FeatureTyping")
				assertRelFeature(t, m, rel, "target", nestedSym(t, root, "T::Sig"))
			}
		}
		rels := m.ImplicitRelationships(parts.payload)
		if len(rels) != 1 {
			t.Fatalf("%s: the payload owns %d written relationships, want its typing", name, len(rels))
		}
		assertRelFeature(t, m, rels[0], "source", parts.payload)
		assertRelFeature(t, m, rels[0], "type", nestedSym(t, root, "T::Sig"))
	}
}
