package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A `then x;` without a body is a SuccessionAsUsage its action owns through a FeatureMembership,
// as the pilot exports it and the RDF writer states it — beside the actions and the `first` Membership.
func TestThenSuccessionIsAnOwnedMember(t *testing.T) {
	const src = `package P {
		action observe {
			first start;
			then action acquire;
			then action readout;
			then done;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "then-succession.sysml", source.KindSysML, src) {
		model := fixture.model
		observe := nestedSym(t, fixture.root, "P::observe")
		var kinds []string
		for _, rel := range model.OwnedRelationships(observe) {
			kinds = append(kinds, relMetaName(model, rel))
			if member := rel.Implicit.Member; member != nil && relMetaName(model, rel) == "FeatureMembership" {
				if meta := model.MetaclassOf(member); meta != nil {
					kinds[len(kinds)-1] += ":" + meta.Name
				}
			}
		}
		want := []string{"Membership", "FeatureMembership:SuccessionAsUsage", "FeatureMembership:ActionUsage", "FeatureMembership:SuccessionAsUsage", "FeatureMembership:ActionUsage", "FeatureMembership:SuccessionAsUsage"}
		if len(kinds) != len(want) {
			t.Fatalf("observe ownedRelationship = %v, want %v", kinds, want)
		}
		for i := range want {
			if kinds[i] != want[i] {
				t.Errorf("observe ownedRelationship[%d] = %s, want %s (all %v)", i, kinds[i], want[i], kinds)
			}
		}
	}
}

// A `then done;` relates the member before it to the library's done the action inherits; a `then assign …;`
// reaches a statement no element stands for yet, so its relatedFeature is unanswered rather than one short.
func TestThenSuccessionRelatedFeatures(t *testing.T) {
	const src = `package P {
		action def Settle { out t : Real; first start; then assign t := 1.0; then done; }
		action observe { first start; then action acquire; then done; }
	}`
	for _, fixture := range reflectiveFixtures(t, "then-related.sysml", source.KindSysML, src) {
		model := fixture.model
		var successions []*symbols.Symbol
		for _, owner := range []string{"P::Settle", "P::observe"} {
			for _, rel := range model.OwnedRelationships(nestedSym(t, fixture.root, owner)) {
				if member := rel.Implicit.Member; member != nil && model.MetaclassOf(member) != nil && model.MetaclassOf(member).Name == "SuccessionAsUsage" {
					successions = append(successions, member)
				}
			}
		}
		if len(successions) != 4 {
			t.Fatalf("%d successions, want 4", len(successions))
		}
		names := func(sym *symbols.Symbol) (string, bool) {
			related, ok := model.ReflectiveElements(sym, "relatedFeature")
			var out []string
			for _, r := range related {
				out = append(out, r.Name)
			}
			return strings.Join(out, " "), ok
		}
		for i, want := range []struct {
			related string
			ok      bool
		}{{"", false}, {"", false}, {"start acquire", true}, {"acquire done", true}} {
			got, ok := names(successions[i])
			if ok != want.ok || ok && got != want.related {
				t.Errorf("succession %d relatedFeature = %q, %v; want %q, %v", i, got, ok, want.related, want.ok)
			}
		}
	}
}

// A `then x;` whose source is the member before it owns its two ends through an EndFeatureMembership
// each, as the RDF writer mints them; `then idle;` after a bare `entry;` reaches its source by position and owns none.
func TestThenSuccessionOwnsItsEnds(t *testing.T) {
	const src = `package P {
		action observe { first start; then action acquire; then done; }
		state def S { entry; then idle; state idle; }
	}`
	for f, fixture := range reflectiveFixtures(t, "then-ends.sysml", source.KindSysML, src) {
		model := fixture.model
		ends := []string{"EndFeatureMembership:ReferenceUsage", "EndFeatureMembership:ReferenceUsage"}
		if f == 1 {
			// A record keeps a connector's ends by position (recordedPositions), as for `connect a to b`.
			ends = nil
		}
		var successions []*symbols.Symbol
		for _, owner := range []string{"P::observe", "P::S"} {
			for _, rel := range model.OwnedRelationships(nestedSym(t, fixture.root, owner)) {
				if member := rel.Implicit.Member; member != nil && model.MetaclassOf(member) != nil && model.MetaclassOf(member).Name == "SuccessionAsUsage" {
					successions = append(successions, member)
				}
			}
		}
		if len(successions) != 3 {
			t.Fatalf("%d successions, want 3", len(successions))
		}
		for i, want := range [][]string{ends, ends, nil} {
			var kinds []string
			for _, rel := range model.OwnedRelationships(successions[i]) {
				name := relMetaName(model, rel)
				if name != "EndFeatureMembership" {
					continue
				}
				end := rel.Implicit.Member
				kinds = append(kinds, name+":"+model.MetaclassOf(end).Name)
				if isEnd, ok := model.ReflectiveFeatureValue(end, "isEnd"); !ok || !isEnd.Bool {
					t.Errorf("succession %d end %d isEnd = %v, %v; want true", i, len(kinds)-1, isEnd, ok)
				}
				// Nameless, each end is named by the happensBeforeLinks end it redefines (KerML 8.3.3.3).
				if name, wantName := model.EffectiveNameOf(end), []string{"earlierOccurrence", "laterOccurrence"}[len(kinds)-1]; name != wantName {
					t.Errorf("succession %d end %d name = %q, want %q", i, len(kinds)-1, name, wantName)
				}
				// The target end reference-subsets the member the succession reaches (acquire, the library's
				// done); the source end is minted empty, as the conversion writes it.
				var referenced []string
				subsettings, _ := model.ReflectiveElements(end, "ownedReferenceSubsetting")
				for _, subsetting := range subsettings {
					targets, _ := model.ReflectiveElements(subsetting, "referencedFeature")
					for _, target := range targets {
						referenced = append(referenced, target.Name)
					}
				}
				want := [][]string{{"acquire"}, {"done"}}[i]
				if len(kinds) == 1 {
					want = nil
				}
				if strings.Join(referenced, ",") != strings.Join(want, ",") {
					t.Errorf("succession %d end %d references %v, want %v", i, len(kinds)-1, referenced, want)
				}
			}
			if strings.Join(kinds, " ") != strings.Join(want, " ") {
				t.Errorf("succession %d end memberships = %v, want %v", i, kinds, want)
			}
		}
	}
}

// TestThenSuccessionEndTakesItsWrittenRange: `then [0..1] action c;` writes the range on the source end the
// succession owns, as the conversion and the pilot state it; that end implies no [1..1], the target end does.
func TestThenSuccessionEndTakesItsWrittenRange(t *testing.T) {
	const src = `package P { action observe { action a; then [0..1] action c; } }`
	fixture := reflectiveFixtures(t, "then-end-range.sysml", source.KindSysML, src)[0]
	model := fixture.model
	var ends []*symbols.Symbol
	for _, rel := range model.OwnedRelationships(nestedSym(t, fixture.root, "P::observe")) {
		if member := rel.Implicit.Member; member != nil {
			ends = append(ends, model.SuccessionEndMembers(member)...)
		}
	}
	if len(ends) != 2 {
		t.Fatalf("%d succession ends, want 2", len(ends))
	}
	if r, ok := model.MultiplicityOf(ends[0]); !ok || r.Lower.Value != 0 || r.Upper.Value != 1 {
		t.Errorf("source end multiplicity = %+v, %v; want [0..1]", r, ok)
	}
	if model.ImpliesDefaultMultiplicity(ends[0]) {
		t.Error("source end with a written range implies the default [1..1]")
	}
	if _, ok := model.MultiplicityOf(ends[1]); ok || !model.ImpliesDefaultMultiplicity(ends[1]) {
		t.Errorf("target end states a range (%v) or no default [1..1]", ok)
	}
}
