package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A flow owns its source and target FlowEnds through an EndFeatureMembership each, after its payload; an end
// reference-subsets the feature before the last it names and owns the ReferenceUsage redefining that last one.
func TestFlowOwnsItsEnds(t *testing.T) {
	const src = `package P {
    item def Photon;
    part def Inner { out port op; in port ip; }
    part def Src { part inner : Inner; out item photons : Photon; }
    part def Snk { part inner : Inner; in item photons : Photon; }
    part src : Src;
    part snk : Snk;
    flow f1 from src.photons to snk.photons;
    flow of p : Photon from src.photons to snk.photons;
    flow g1 from src.inner.op to snk.inner.ip;
    message m1 from src.photons to snk.photons;
}`
	for _, fixture := range reflectiveFixtures(t, "flow-ends.sysml", source.KindSysML, src) {
		model := fixture.model
		if nestedSym(t, fixture.root, "P::src").Recorded() {
			// A recorded symbol keeps no flow ends.
			continue
		}
		ends := func(flow *symbols.Symbol) string {
			var parts []string
			for _, end := range ownedMembers(model, flow) {
				_, implied := symbols.ImpliedFlowEndIndex(end)
				if !implied {
					continue
				}
				parts = append(parts, model.EffectiveNameOf(end)+"{"+ownedMemberKinds(model, end)+"}")
				for _, rel := range model.OwnedRelationships(end) {
					if feature := rel.Implicit.Member; feature != nil && feature.Kind == symbols.SymbolReferenceUsage && feature.Chain == nil {
						parts = append(parts, model.EffectiveNameOf(feature)+"{"+ownedMemberKinds(model, feature)+"}")
					}
				}
			}
			return strings.Join(parts, " ")
		}
		f1 := nestedSym(t, fixture.root, "P::f1")
		if got, want := ownedMemberKinds(model, f1), "EndFeatureMembership:FlowEnd EndFeatureMembership:FlowEnd"; got != want {
			t.Errorf("f1's ownedRelationship = %s, want %s", got, want)
		}
		want := "source{ReferenceSubsetting:PartUsage FeatureMembership:ReferenceUsage} photons{Redefinition:ItemUsage} " +
			"target{ReferenceSubsetting:PartUsage FeatureMembership:ReferenceUsage} photons{Redefinition:ItemUsage}"
		if got := ends(f1); got != want {
			t.Errorf("f1's ends = %s, want %s", got, want)
		}
		for _, member := range ownedMembers(model, nestedSym(t, fixture.root, "P")) {
			if member.Name != "" || member.Kind != symbols.SymbolFlowUsage {
				continue
			}
			if got, want := ownedMemberKinds(model, member), "FeatureMembership:PayloadFeature EndFeatureMembership:FlowEnd EndFeatureMembership:FlowEnd"; got != want {
				t.Errorf("the payload flow's ownedRelationship = %s, want %s", got, want)
			}
		}
		g1 := nestedSym(t, fixture.root, "P::g1")
		want = "source{ReferenceSubsetting:Feature FeatureMembership:ReferenceUsage} op{Redefinition:PortUsage} " +
			"target{ReferenceSubsetting:Feature FeatureMembership:ReferenceUsage} ip{Redefinition:PortUsage}"
		if got := ends(g1); got != want {
			t.Errorf("g1's ends = %s, want %s", got, want)
		}
		for _, end := range model.FlowEndMembers(g1) {
			for _, rel := range model.OwnedRelationships(end) {
				chain := rel.Implicit.Member
				if chain == nil || chain.Chain == nil {
					continue
				}
				if owned, _ := model.ReflectiveElements(rel, "ownedRelatedElement"); len(owned) != 1 || owned[0] != chain {
					t.Errorf("g1's end %s: ReferenceSubsetting's ownedRelatedElement = %v, want the chain Feature it references", model.EffectiveNameOf(end), owned)
				}
				if owning := model.OwningRelationship(chain); owning != rel {
					t.Errorf("g1's end %s: the chain Feature's owningRelationship = %v, want the end's ReferenceSubsetting", model.EffectiveNameOf(end), owning)
				}
			}
		}
		for _, end := range model.EndFeatures(g1) {
			if !model.ImpliesDefaultMultiplicity(end) {
				continue
			}
			t.Errorf("g1's end %s implies a default multiplicity; a FlowEnd states none", model.EffectiveNameOf(end))
		}
		// A message owns no FlowEnd: its ends are the event occurrences it relates, held as
		// parameters (SysML.xtext MessageEventMember), as the conversion writes them.
		want = "ParameterMembership:EventOccurrenceUsage ParameterMembership:EventOccurrenceUsage"
		if got := ownedMemberKinds(model, nestedSym(t, fixture.root, "P::m1")); got != want {
			t.Errorf("the message's ownedRelationship = %s, want %s", got, want)
		}
	}
}

func TestFlowEndsAnswerTheirSiblingSurfaces(t *testing.T) {
	const src = `package P {
    item def Photon;
    part def A { out x : Photon; }
    part def B { in y : Photon; }
    part a : A;
    part b : B;
    flow fp of p : Photon from a.x to b.y;
    flow f3 from a.x to b.y;
    flow f4 :> f3;
}`
	for _, fixture := range reflectiveFixtures(t, "flow-end-surfaces.sysml", source.KindSysML, src) {
		model := fixture.model
		if nestedSym(t, fixture.root, "P::a").Recorded() {
			continue
		}
		f3, f4 := nestedSym(t, fixture.root, "P::f3"), nestedSym(t, fixture.root, "P::f4")
		if got := model.RelatedFeatureCount(f4); got != 2 {
			t.Errorf("f4 :> f3 RelatedFeatureCount = %d, want 2 (f3's %d), the ends it inherits", got, model.RelatedFeatureCount(f3))
		}
		fp := nestedSym(t, fixture.root, "P::fp")
		memberships := effectiveNames(model, ownedMembershipMembers(model, fp))
		if want := "p source target"; strings.Join(memberships, " ") != want {
			t.Errorf("fp ownedMembership members = %v, want %q: the ends after the payload", memberships, want)
		}
		var relationships []string
		for _, rel := range model.OwnedRelationships(fp) {
			relationships = append(relationships, model.EffectiveNameOf(rel.Implicit.Member))
		}
		if want := "p source target"; strings.Join(relationships, " ") != want {
			t.Errorf("fp ownedRelationship members = %v, want %q", relationships, want)
		}
		for _, end := range model.FlowEndMembers(fp) {
			feature := model.FlowEndFeatureMember(end)
			for _, prop := range []string{"feature", "ownedElement", "ownedMember", "member", "ownedFeature"} {
				if vals, _ := model.ReflectiveElements(end, prop); !containsSymbol(vals, feature) {
					t.Errorf("end %s %s lacks the ReferenceUsage it owns", model.EffectiveNameOf(end), prop)
				}
			}
			if types, _ := model.ReflectiveElements(feature, "type"); strings.Join(effectiveNames(model, types), " ") != "Photon" {
				t.Errorf("end %s's %s type = %v, want Photon, through the feature it redefines", model.EffectiveNameOf(end), model.EffectiveNameOf(feature), effectiveNames(model, types))
			}
		}
	}
}

func effectiveNames(model *Model, syms []*symbols.Symbol) []string {
	out := make([]string, 0, len(syms))
	for _, sym := range syms {
		out = append(out, model.EffectiveNameOf(sym))
	}
	return out
}

func ownedMembershipMembers(model *Model, sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, membership := range model.OwnedMemberships(sym) {
		out = append(out, membership.Implicit.Member)
	}
	return out
}

func containsSymbol(syms []*symbols.Symbol, want *symbols.Symbol) bool {
	for _, sym := range syms {
		if sym == want {
			return true
		}
	}
	return false
}

// The ReferenceUsage a flow's end owns states the direction of the feature it redefines as the feature the
// end references inherits it (KerML.xtext FlowFeature, note 1); an end naming a lone feature states none.
func TestFlowEndFeatureStatesItsRedefinedDirection(t *testing.T) {
	const src = `package P {
    item def Photon;
    port def Lamp { out item photons : Photon; }
    part def Src { out item photons : Photon; port lamp : ~Lamp; }
    part def Snk { in item photons : Photon; }
    part src : Src;
    part snk : Snk;
    item loose : Photon;
    flow f1 from src.photons to snk.photons;
    flow f2 from src.lamp.photons to loose;
}`
	for _, fixture := range reflectiveFixtures(t, "flow-end-directions.sysml", source.KindSysML, src) {
		model := fixture.model
		if nestedSym(t, fixture.root, "P::src").Recorded() {
			continue
		}
		directions := func(name string) string {
			var parts []string
			for _, end := range model.FlowEndMembers(nestedSym(t, fixture.root, name)) {
				value, ok := model.ReflectiveFeatureValue(model.FlowEndFeatureMember(end), "direction")
				switch {
				case !ok:
					parts = append(parts, "?")
				case value.Kind == symbols.FilterValueEmpty:
					parts = append(parts, "none")
				default:
					parts = append(parts, value.RefFQN[strings.LastIndex(value.RefFQN, ":")+1:])
				}
			}
			return strings.Join(parts, " ")
		}
		if got, want := directions("P::f1"), "out in"; got != want {
			t.Errorf("f1's end features state %s, want %s", got, want)
		}
		// The conjugated port reverses the direction its definition declares.
		if got, want := directions("P::f2"), "in none"; got != want {
			t.Errorf("f2's end features state %s, want %s", got, want)
		}
	}
}
