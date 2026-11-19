package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The succession a transition owns holds its two ends as a `then` succession does (TestThenSuccessionOwnsItsEnds):
// nameless ReferenceUsages through an EndFeatureMembership each, named by the end they redefine, the target end
// reference-subsetting the target state and the source end minted empty — a `transition` and a guarded target alike.
func TestTransitionSuccessionOwnsItsEnds(t *testing.T) {
	const src = `package P {
		state def S {
			state idle;
			state busy;
			transition go first idle then busy;
			transition if true then idle;
		}
	}`
	for f, fixture := range reflectiveFixtures(t, "transition-ends.sysml", source.KindSysML, src) {
		model := fixture.model
		var successions []*symbols.Symbol
		for _, member := range ownedMembers(model, nestedSym(t, fixture.root, "P::S")) {
			if meta := model.MetaclassOf(member); meta != nil && meta.Name == "TransitionUsage" {
				owned, _ := model.ReflectiveElements(member, "succession")
				successions = append(successions, owned...)
			}
		}
		if f == 1 {
			// A record keeps a transition's parts out of its walk (TestStateBodyTransitionMintsNoSuccessionEnds).
			if len(successions) != 0 {
				t.Fatalf("recorded fixture: %d successions, want 0", len(successions))
			}
			continue
		}
		if len(successions) != 2 {
			t.Fatalf("%d transition successions, want 2", len(successions))
		}
		for i, wantTarget := range []string{"busy", "idle"} {
			var kinds []string
			for _, rel := range model.OwnedRelationships(successions[i]) {
				end := rel.Implicit.Member
				kinds = append(kinds, relMetaName(model, rel)+":"+model.MetaclassOf(end).Name)
				if isEnd, ok := model.ReflectiveFeatureValue(end, "isEnd"); !ok || !isEnd.Bool {
					t.Errorf("succession %d end %d isEnd = %v, %v; want true", i, len(kinds)-1, isEnd, ok)
				}
				if name, wantName := model.EffectiveNameOf(end), []string{"earlierOccurrence", "laterOccurrence"}[len(kinds)-1]; name != wantName {
					t.Errorf("succession %d end %d name = %q, want %q", i, len(kinds)-1, name, wantName)
				}
				var referenced []string
				subsettings, _ := model.ReflectiveElements(end, "ownedReferenceSubsetting")
				for _, subsetting := range subsettings {
					targets, _ := model.ReflectiveElements(subsetting, "referencedFeature")
					for _, target := range targets {
						referenced = append(referenced, target.Name)
					}
				}
				want := ""
				if len(kinds) == 2 {
					want = wantTarget
				}
				if strings.Join(referenced, ",") != want {
					t.Errorf("succession %d end %d references %v, want %q", i, len(kinds)-1, referenced, want)
				}
			}
			if got, want := strings.Join(kinds, " "), "EndFeatureMembership:ReferenceUsage EndFeatureMembership:ReferenceUsage"; got != want {
				t.Errorf("succession %d end memberships = %q, want %q", i, got, want)
			}
		}
	}
}
