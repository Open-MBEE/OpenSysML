package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A metadata body's `mission = "Hubble"` is a ReferenceUsage its MetadataUsage — an
// occurrence — owns, so it may time-vary and is variable, as the pilot exports it.
func TestMetadataBodyFeatureIsVariable(t *testing.T) {
	const src = `package P {
		part def T; part telescope : T;
		metadata def Heritage { attribute mission : String; }
		@Heritage about telescope { mission = "Hubble"; }
	}`
	for _, fixture := range reflectiveFixtures(t, "metadata-body-variable.sysml", source.KindSysML, src) {
		model := fixture.model
		if nestedSym(t, fixture.root, "P::telescope").Recorded() {
			continue
		}
		var body []*symbols.Symbol
		for _, member := range model.Members(nestedSym(t, fixture.root, "P")) {
			if member.Symbol.Kind == symbols.SymbolMetadataUsage {
				body = ownedBodyMembers(model, member.Symbol)
			}
		}
		if len(body) != 1 {
			t.Fatalf("metadata body members = %d, want mission", len(body))
		}
		mission := body[0]
		if !model.UsageMayTimeVary(mission) {
			t.Errorf("mission mayTimeVary = false, want true: its owning MetadataUsage is an occurrence")
		}
		if !model.FeatureIsVariable(mission) {
			t.Errorf("mission isVariable = false, want true")
		}
	}
}
