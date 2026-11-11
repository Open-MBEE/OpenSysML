package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A state body's `then x;` after a state is a TargetTransitionUsage (SysML v2.1 §8.2.2.18.1, §8.2.2.18.3): a
// TransitionUsage the state owns, owning the SuccessionAsUsage; `entry; then x;` and `then state s;` stay successions.
func TestStateBodyThenAfterStateIsATransitionUsage(t *testing.T) {
	const src = `package P {
		state def Mach {
			entry; then idle;
			state idle; then running;
			state running { entry; then inner; state inner; then fin; state fin; }
			then state off;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "state-then.sysml", source.KindSysML, src) {
		model := fixture.model
		mach := nestedSym(t, fixture.root, "P::Mach")
		got := ownedMemberKinds(model, mach)
		want := "StateSubactionMembership:ActionUsage FeatureMembership:SuccessionAsUsage FeatureMembership:StateUsage FeatureMembership:TransitionUsage FeatureMembership:StateUsage FeatureMembership:SuccessionAsUsage FeatureMembership:StateUsage"
		if got != want {
			t.Fatalf("Mach ownedRelationship = %s\nwant %s", got, want)
		}
		transition := ownedMembers(model, mach)[3]
		if transition.Recorded() {
			// A record carries the metaclass but no declaration to read the transition's parts from,
			// as for a transition's trigger and guard.
			continue
		}
		if got, want := ownedMemberKinds(model, transition), "OwningMembership:SuccessionAsUsage"; got != want {
			t.Errorf("the transition's ownedRelationship = %s, want %s", got, want)
		}
		succession := ownedMembers(model, transition)[0]
		if got, ok := relatedNames(model, succession); !ok || got != "idle running" {
			t.Errorf("the transition's succession relatedFeature = %q, %v; want \"idle running\", true", got, ok)
		}
		if owned, _ := model.ReflectiveElements(transition, "succession"); len(owned) != 1 || owned[0] != succession {
			t.Errorf("transition.succession = %v, want the owned succession", owned)
		}
		if got, want := ownedMemberKinds(model, ownedMembers(model, mach)[1]), ""; got != want {
			t.Errorf("the entry transition's succession owns %s, want nothing", got)
		}
		running := ownedMembers(model, mach)[4]
		if got, want := ownedMemberKinds(model, running), "StateSubactionMembership:ActionUsage FeatureMembership:SuccessionAsUsage FeatureMembership:StateUsage FeatureMembership:TransitionUsage FeatureMembership:StateUsage"; got != want {
			t.Errorf("running ownedRelationship = %s\nwant %s", got, want)
		}
		inner := ownedMembers(model, running)[3]
		if got, ok := relatedNames(model, ownedMembers(model, inner)[0]); !ok || got != "inner fin" {
			t.Errorf("inner's transition succession relatedFeature = %q, %v; want \"inner fin\", true", got, ok)
		}
	}
}

// ownedMemberKinds spells sym's ownedRelationships as `<Membership>:<member metaclass>`, in order.
func ownedMemberKinds(model *Model, sym *symbols.Symbol) string {
	var kinds []string
	for _, rel := range model.OwnedRelationships(sym) {
		kind := relMetaName(model, rel)
		if member := rel.Implicit.Member; member != nil {
			if meta := model.MetaclassOf(member); meta != nil {
				kind += ":" + meta.Name
			}
		}
		kinds = append(kinds, kind)
	}
	return strings.Join(kinds, " ")
}

// ownedMembers are the members of sym's ownedRelationships, in order.
func ownedMembers(model *Model, sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range model.OwnedRelationships(sym) {
		out = append(out, rel.Implicit.Member)
	}
	return out
}

// relatedNames are the names of sym's relatedFeatures, and whether the model answers them.
func relatedNames(model *Model, sym *symbols.Symbol) (string, bool) {
	related, ok := model.ReflectiveElements(sym, "relatedFeature")
	var out []string
	for _, r := range related {
		out = append(out, r.Name)
	}
	return strings.Join(out, " "), ok
}
