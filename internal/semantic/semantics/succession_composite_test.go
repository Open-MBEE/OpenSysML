package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A succession or binding is a reference whether written `then x;`, `succession s first a then b;` or
// `bind a = b;` (SysML § 7.6.3; the RDF writer's nonCompositeUsageClasses, as the pilot exports them).
func TestReflectiveSuccessionAndBindingAreNotComposite(t *testing.T) {
	const src = `package P {
		action observe {
			first start;
			then action acquire;
			then done;
			succession s first acquire then done;
		}
		part p { part a; part b; bind a = b; }
	}`
	for _, fixture := range reflectiveFixtures(t, "succession-composite.sysml", source.KindSysML, src) {
		model := fixture.model
		var members []*symbols.Symbol
		for _, owner := range []string{"P::observe", "P::p"} {
			for _, rel := range model.OwnedRelationships(nestedSym(t, fixture.root, owner)) {
				member := rel.Implicit.Member
				if member == nil || model.MetaclassOf(member) == nil {
					continue
				}
				switch model.MetaclassOf(member).Name {
				case "SuccessionAsUsage", "BindingConnectorAsUsage":
					members = append(members, member)
				}
			}
		}
		if len(members) != 4 {
			t.Fatalf("%d succession/binding members, want 4", len(members))
		}
		for _, member := range members {
			for feature, want := range map[string]bool{"isComposite": false, "isReference": true} {
				got, ok := model.ReflectiveFeatureValue(member, feature)
				if !ok || got.Bool != want {
					t.Errorf("%s %s.%s = %v (present %t), want %t", model.MetaclassOf(member).Name, symbols.FQNOf(member), feature, got, ok, want)
				}
			}
		}
	}
}
