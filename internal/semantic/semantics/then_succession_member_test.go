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
