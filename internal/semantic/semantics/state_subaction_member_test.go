package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A bare `entry;`, `do;` or `exit;` performs an anonymous empty ActionUsage the state owns through a
// StateSubactionMembership of that kind (SysML.xtext EmptyActionUsage), the `then` after it sourced there.
func TestBareStateSubactionIsTheActionUsageItsMembershipOwns(t *testing.T) {
	const src = `package P {
		state def S { entry; then idle; state idle; do; exit; }
	}`
	for _, fixture := range reflectiveFixtures(t, "state-subactions.sysml", source.KindSysML, src) {
		model := fixture.model
		var actions, successions []*symbols.Symbol
		for _, rel := range model.OwnedRelationships(nestedSym(t, fixture.root, "P::S")) {
			member := rel.Implicit.Member
			if member == nil {
				continue
			}
			meta := model.MetaclassOf(member)
			if meta == nil {
				continue
			}
			switch meta.Name {
			case "ActionUsage":
				actions = append(actions, member)
				if got := relMetaName(model, rel); got != "StateSubactionMembership" {
					t.Errorf("action owned through %s, want StateSubactionMembership", got)
				}
			case "SuccessionAsUsage":
				successions = append(successions, member)
			}
		}
		if len(actions) != 3 || len(successions) != 1 {
			t.Fatalf("S owns %d actions and %d successions, want 3 and 1", len(actions), len(successions))
		}
		for i, kind := range []string{"entry", "do", "exit"} {
			if got := stateSubactionKind(actions[i]); got != kind {
				t.Errorf("subaction %d kind = %q, want %q", i, got, kind)
			}
			if actions[i].Name != "" {
				t.Errorf("subaction %d is named %q, want anonymous", i, actions[i].Name)
			}
		}
		source, ok := model.ReflectiveElements(successions[0], "sourceFeature")
		if !ok || len(source) != 1 || source[0] != actions[0] {
			t.Errorf("the entry succession's sourceFeature = %v (%t), want the entry action", source, ok)
		}
		related, ok := model.ReflectiveElements(successions[0], "relatedFeature")
		if !ok || len(related) != 2 || related[0] != actions[0] || related[1].Name != "idle" {
			t.Errorf("the entry succession's relatedFeature = %v (%t), want [entry action, idle]", related, ok)
		}
	}
}
