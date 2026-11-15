package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A metadata usage owns the FeatureTyping its `@M` or `#M` states, ahead of its annotation and
// body, as the conversion writes it (`_ft0`) and the pilot exports it.
func TestMetadataUsageOwnsItsTyping(t *testing.T) {
	const src = `package P {
    metadata def Heritage { attribute mission : String; }
    part telescope;
    @Heritage about telescope { mission = "Hubble"; }
    part camera { @Heritage { mission = "Webb"; } }
    metadata def <flight> FlightMetadata :> Metaobjects::SemanticMetadata { :>> baseType = parts meta SysML::Usage; }
    #flight part mirror;
}`
	for _, fixture := range reflectiveFixtures(t, "metadata-typing.sysml", source.KindSysML, src) {
		model := fixture.model
		if nestedSym(t, fixture.root, "P::telescope").Recorded() {
			// A recorded symbol keeps no annotation or body.
			continue
		}
		var about, body, prefix string
		for _, member := range ownedMembers(model, nestedSym(t, fixture.root, "P")) {
			if member.Name == "" {
				about = ownedMemberKinds(model, member)
			}
		}
		for _, member := range ownedMembers(model, nestedSym(t, fixture.root, "P::camera")) {
			if member.Name == "" {
				body = ownedMemberKinds(model, member)
			}
		}
		for _, member := range ownedMembers(model, nestedSym(t, fixture.root, "P::mirror")) {
			if member.Name == "" {
				prefix = ownedMemberKinds(model, member)
			}
		}
		if want := "FeatureTyping Annotation:PartUsage FeatureMembership:ReferenceUsage"; about != want {
			t.Errorf("the about metadata's ownedRelationship = %s, want %s", about, want)
		}
		if want := "FeatureTyping FeatureMembership:ReferenceUsage"; body != want {
			t.Errorf("the body metadata's ownedRelationship = %s, want %s", body, want)
		}
		if want := "FeatureTyping"; prefix != want {
			t.Errorf("the `#flight` prefix metadata's ownedRelationship = %s, want %s", prefix, want)
		}
	}
}
