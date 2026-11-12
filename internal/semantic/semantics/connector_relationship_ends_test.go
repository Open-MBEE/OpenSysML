package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A connector's relatedElement, source and target are the features its relatedFeature, sourceFeature and
// targetFeature redefine them as (KerML 8.3.4.5.1); a `then idle;` after `entry;` sources from the entry's action.
func TestConnectorRelationshipEndsAreItsRelatedFeatures(t *testing.T) {
	const src = `package P {
		action observe {
			first start;
			then action acquire { out image : Real; }
			then action readout { in image : Real; }
			then done;
			flow acquire.image to readout.image;
		}
		state ts { entry; then idle; state idle; }
	}`
	for _, fixture := range reflectiveFixtures(t, "connector-ends.sysml", source.KindSysML, src) {
		model := fixture.model
		connectors := func(owner string) []*symbols.Symbol {
			var out []*symbols.Symbol
			for _, rel := range model.OwnedRelationships(nestedSym(t, fixture.root, owner)) {
				if member := rel.Implicit.Member; member != nil && model.reflectiveMetaclassConforms(member, "Connector") {
					out = append(out, member)
				}
			}
			return out
		}
		names := func(sym *symbols.Symbol, property string) (string, bool) {
			elements, ok := model.ReflectiveElements(sym, property)
			var out []string
			for _, e := range elements {
				out = append(out, e.Name)
			}
			return strings.Join(out, " "), ok
		}
		observe := connectors("P::observe")
		if len(observe) != 4 {
			t.Fatalf("observe owns %d connectors, want 3 successions and a flow", len(observe))
		}
		for i, want := range []struct{ related, source, target string }{
			{"start acquire", "start", "acquire"}, {"acquire readout", "acquire", "readout"},
			{"readout done", "readout", "done"}, {"image image", "image", "image"},
		} {
			for property, w := range map[string]string{"relatedElement": want.related, "source": want.source, "target": want.target} {
				if got, ok := names(observe[i], property); !ok || got != w {
					t.Errorf("connector %d %s = %q, %v; want %q", i, property, got, ok, w)
				}
			}
		}
		entry := connectors("P::ts")
		if len(entry) != 1 {
			t.Fatalf("ts owns %d connectors, want the entry succession", len(entry))
		}
		for property, want := range map[string]string{"relatedElement": " idle", "source": "", "target": "idle"} {
			if got, ok := names(entry[0], property); !ok || got != want {
				t.Errorf("entry succession %s = %q (%v), want %q: the anonymous entry action and idle", property, got, ok, want)
			}
		}
	}
}

// An association's relatedElement, source and target are the relatedType, sourceType and targetType that
// redefine them (KerML 8.3.4.4.2): its ends' types in order, the first the source, as the pilot derives them.
func TestAssociationRelationshipEndsAreItsEndTypes(t *testing.T) {
	const src = `package P {
		part def A;
		part def B;
		action def Observe;
		allocation def F2P { end function : Observe; end part : A; }
		connection def C { end a : A; end b : B; end c : B; }
	}`
	for _, fixture := range reflectiveFixtures(t, "association-ends.sysml", source.KindSysML, src) {
		model := fixture.model
		for owner, want := range map[string]struct{ related, source, target string }{
			"P::F2P": {"Observe A", "Observe", "A"},
			"P::C":   {"A B B", "A", "B B"},
		} {
			sym := nestedSym(t, fixture.root, owner)
			for property, w := range map[string]string{"relatedElement": want.related, "source": want.source, "target": want.target} {
				elements, ok := model.ReflectiveElements(sym, property)
				var got []string
				for _, e := range elements {
					got = append(got, e.Name)
				}
				if !ok || strings.Join(got, " ") != w {
					t.Errorf("%s %s = %q, %v; want %q", owner, property, got, ok, w)
				}
			}
		}
	}
}
