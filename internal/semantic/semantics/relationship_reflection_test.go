package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// reflectiveElementsOf is ReflectiveElements(sym, feature), which must derive.
func reflectiveElementsOf(t *testing.T, model *Model, sym *symbols.Symbol, feature string) []*symbols.Symbol {
	t.Helper()
	got, ok := model.ReflectiveElements(sym, feature)
	if !ok {
		t.Fatalf("%s.%s is not derived", model.fqnOf(sym), feature)
	}
	return got
}

// singleRelationship is the one relationship sym owns under property, classified
// by metaclass, whose source is sym; it returns the relationship and its target.
func singleRelationship(t *testing.T, model *Model, sym *symbols.Symbol, property, metaclass string) (rel, target *symbols.Symbol) {
	t.Helper()
	rels := reflectiveElementsOf(t, model, sym, property)
	if len(rels) != 1 {
		t.Fatalf("%s.%s has %d relationships, want 1", model.fqnOf(sym), property, len(rels))
	}
	rel = rels[0]
	if meta := model.metaclassOf(rel); model.fqnOf(meta) != metaclass {
		t.Errorf("%s.%s is a %s, want %s", model.fqnOf(sym), property, model.fqnOf(meta), metaclass)
	}
	if sources := reflectiveElementsOf(t, model, rel, "source"); len(sources) != 1 || sources[0] != sym {
		t.Errorf("%s.%s.source = %v, want %s", model.fqnOf(sym), property, fqns(sources), model.fqnOf(sym))
	}
	targets := reflectiveElementsOf(t, model, rel, "target")
	if len(targets) != 1 {
		t.Fatalf("%s.%s.target has %d elements, want 1", model.fqnOf(sym), property, len(targets))
	}
	related := reflectiveElementsOf(t, model, rel, "relatedElement")
	if len(related) != 2 || related[0] != sym || related[1] != targets[0] {
		t.Errorf("%s.%s.relatedElement = %v, want the source then the target", model.fqnOf(sym), property, fqns(related))
	}
	return rel, targets[0]
}

// TestReflectiveConnectorEndsAreOwnedMembers: a connector usage owns its ends,
// named or not, as end features and members (SysML v2 8.2.2.13.1), and each
// end's attachment is a ReferenceSubsetting it owns whose target is the chain
// written, with the chain's features as chainingFeature.
func TestReflectiveConnectorEndsAreOwnedMembers(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		port def Port { attribute value : Real; }
		part def Source { port y : Port; }
		part def Sink { port u : Port; }
		connection def C { end source : Port; end target : Port; }
		part def Asm {
			part s : Source;
			part k : Sink;
			connection c1 : C connect [1] s.y to [1] k.u;
			connection c2 connect first references s.y to second references k.u;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "connector-ends.sysml", source.KindSysML, src) {
		asm := nestedSym(t, fixture.root, "P::Asm")
		for _, name := range []string{"c1", "c2"} {
			connector := nestedSym(t, fixture.root, "P::Asm::"+name)
			ends := reflectiveElementsOf(t, fixture.model, connector, "ownedEndFeature")
			if len(ends) != 2 {
				t.Fatalf("%s.ownedEndFeature has %d ends, want 2 (recorded %t)", name, len(ends), connector.Recorded())
			}
			for _, property := range []string{"ownedFeature", "ownedMember"} {
				if got := reflectiveElementsOf(t, fixture.model, connector, property); len(got) != 2 || got[0] != ends[0] || got[1] != ends[1] {
					t.Errorf("%s.%s = %v, want the two ends (recorded %t)", name, property, fqns(got), connector.Recorded())
				}
			}
			// The inherited properties go on with what the connector's types declare.
			for _, property := range []string{"ownedElement", "endFeature", "feature", "member"} {
				if got := reflectiveElementsOf(t, fixture.model, connector, property); len(got) < 2 || got[0] != ends[0] || got[1] != ends[1] {
					t.Errorf("%s.%s = %v, want it to begin with the two ends (recorded %t)", name, property, fqns(got), connector.Recorded())
				}
			}
			for i, end := range ends {
				if meta := fixture.model.metaclassOf(end); fixture.model.fqnOf(meta) != "SysML::Systems::ReferenceUsage" {
					t.Errorf("%s end %d is a %s, want a ReferenceUsage", name, i, fixture.model.fqnOf(meta))
				}
				if owners := reflectiveElementsOf(t, fixture.model, end, "owner"); len(owners) != 1 || owners[0] != connector {
					t.Errorf("%s end %d owner = %v, want %s", name, i, fqns(owners), name)
				}
				if got, want := fixture.model.ownerOf(end), connector; got != want {
					t.Errorf("%s end %d is owned by %s, want %s", name, i, fixture.model.fqnOf(got), name)
				}
			}
			wantTips := []string{"P::Source::y", "P::Sink::u"}
			wantChains := [][]string{{"P::Asm::s", "P::Source::y"}, {"P::Asm::k", "P::Sink::u"}}
			for i, end := range ends {
				rel, target := singleRelationship(t, fixture.model, end, "ownedReferenceSubsetting", "KerML::Core::ReferenceSubsetting")
				if subsetting := reflectiveElementsOf(t, fixture.model, end, "ownedSubsetting"); len(subsetting) != 1 || subsetting[0] != rel {
					t.Errorf("%s end %d ownedSubsetting = %v, want its reference subsetting", name, i, fqns(subsetting))
				}
				if typings := reflectiveElementsOf(t, fixture.model, end, "ownedTyping"); len(typings) != 0 {
					t.Errorf("%s end %d ownedTyping = %v, want none", name, i, fqns(typings))
				}
				if meta := fixture.model.metaclassOf(target); fixture.model.fqnOf(meta) != "KerML::Core::Feature" {
					t.Errorf("%s end %d references a %s, want the chain as a Feature", name, i, fixture.model.fqnOf(meta))
				}
				assertReflectiveElements(t, fixture.model, target, "chainingFeature", wantChains[i]...)
				if ownedRelated := reflectiveElementsOf(t, fixture.model, rel, "ownedRelatedElement"); len(ownedRelated) != 1 || ownedRelated[0] != target {
					t.Errorf("%s end %d reference subsetting owns %v, want its chain feature", name, i, fqns(ownedRelated))
				}
				// The chain feature's owner is the end owning the relationship that
				// owns it; the relationship owns nothing through a relationship of its own.
				if owners := reflectiveElementsOf(t, fixture.model, target, "owner"); len(owners) != 1 || owners[0] != end {
					t.Errorf("%s end %d chain feature owner = %v, want the end", name, i, fqns(owners))
				}
				if owned := reflectiveElementsOf(t, fixture.model, rel, "ownedElement"); len(owned) != 0 {
					t.Errorf("%s end %d reference subsetting ownedElement = %v, want none", name, i, fqns(owned))
				}
			}
			if tips := reflectiveElementsOf(t, fixture.model, connector, "relatedFeature"); len(tips) != 2 ||
				fixture.model.fqnOf(tips[0]) != wantTips[0] || fixture.model.fqnOf(tips[1]) != wantTips[1] {
				t.Errorf("%s.relatedFeature = %v, want %v", name, fqns(tips), wantTips)
			}
		}
		// The ends are not members of the connector's owner.
		for _, member := range reflectiveElementsOf(t, fixture.model, asm, "ownedMember") {
			if member.Kind == symbols.SymbolConnectorEnd {
				t.Errorf("P::Asm owns a connector end %s", fixture.model.fqnOf(member))
			}
		}
	}
}

// TestReflectivePlainUsagesOwnTheirRelationships: a usage's `: T`, `:> s`, `:>> x`
// and `::> r` clauses are Relationship metaobjects it owns, read back through
// the property of their kind and the general `ownedSubsetting`, each with the
// usage as source and the named feature as target.
func TestReflectivePlainUsagesOwnTheirRelationships(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		part def Base {
			attribute x : Real;
			attribute y : Real;
			part sub { attribute w : Real; }
		}
		part def Derived :> Base {
			attribute :>> x = 1.0;
			attribute z : Real :> y;
			ref attribute r ::> y;
			attribute deep :>> sub.w;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "plain-relationships.sysml", source.KindSysML, src) {
		model := fixture.model
		x := nestedSym(t, fixture.root, "P::Derived::x")
		redefinition, target := singleRelationship(t, model, x, "ownedRedefinition", "KerML::Core::Redefinition")
		if model.fqnOf(target) != "P::Base::x" {
			t.Errorf("x redefines %s, want P::Base::x", model.fqnOf(target))
		}
		if subsetting := reflectiveElementsOf(t, model, x, "ownedSubsetting"); len(subsetting) != 1 || subsetting[0] != redefinition {
			t.Errorf("x.ownedSubsetting = %v, want its redefinition", fqns(subsetting))
		}
		for _, property := range []string{"ownedReferenceSubsetting", "ownedTyping"} {
			if got := reflectiveElementsOf(t, model, x, property); len(got) != 0 {
				t.Errorf("x.%s = %v, want none", property, fqns(got))
			}
		}

		z := nestedSym(t, fixture.root, "P::Derived::z")
		_, target = singleRelationship(t, model, z, "ownedSubsetting", "KerML::Core::Subsetting")
		if model.fqnOf(target) != "P::Base::y" {
			t.Errorf("z subsets %s, want P::Base::y", model.fqnOf(target))
		}
		_, target = singleRelationship(t, model, z, "ownedTyping", "KerML::Core::FeatureTyping")
		if model.fqnOf(target) != "ScalarValues::Real" {
			t.Errorf("z is typed by %s, want ScalarValues::Real", model.fqnOf(target))
		}
		if got := reflectiveElementsOf(t, model, z, "ownedRedefinition"); len(got) != 0 {
			t.Errorf("z.ownedRedefinition = %v, want none", fqns(got))
		}

		r := nestedSym(t, fixture.root, "P::Derived::r")
		reference, target := singleRelationship(t, model, r, "ownedReferenceSubsetting", "KerML::Core::ReferenceSubsetting")
		if model.fqnOf(target) != "P::Base::y" {
			t.Errorf("r references %s, want P::Base::y", model.fqnOf(target))
		}
		if subsetting := reflectiveElementsOf(t, model, r, "ownedSubsetting"); len(subsetting) != 1 || subsetting[0] != reference {
			t.Errorf("r.ownedSubsetting = %v, want its reference subsetting", fqns(subsetting))
		}

		deep := nestedSym(t, fixture.root, "P::Derived::deep")
		_, target = singleRelationship(t, model, deep, "ownedRedefinition", "KerML::Core::Redefinition")
		assertReflectiveElements(t, model, target, "chainingFeature", "P::Base::sub", "P::Base::sub::w")

		derived := nestedSym(t, fixture.root, "P::Derived")
		_, target = singleRelationship(t, model, derived, "ownedSubclassification", "KerML::Core::Subclassification")
		if model.fqnOf(target) != "P::Base" {
			t.Errorf("Derived specializes %s, want P::Base", model.fqnOf(target))
		}
		if specializations := reflectiveElementsOf(t, model, derived, "ownedSpecialization"); len(specializations) != 1 {
			t.Errorf("Derived.ownedSpecialization has %d relationships, want 1", len(specializations))
		}
		if _, ok := model.ReflectiveElements(derived, "ownedRedefinition"); ok {
			t.Error("ownedRedefinition derives for a definition, want it only for features")
		}
	}
}
