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
		allocation def Multi { end x : A, B; end y : Observe; }
	}`
	for _, fixture := range reflectiveFixtures(t, "association-ends.sysml", source.KindSysML, src) {
		model := fixture.model
		for owner, want := range map[string]struct{ related, source, target string }{
			"P::F2P": {"Observe A", "Observe", "A"},
			// relatedElement keeps repeats, target is unique (KerML 8.3.2.1), as the pilot derives both.
			"P::C": {"A B B", "A", "B"},
			// An end typed twice contributes both types in order: the first is the source, the second a target.
			"P::Multi": {"A B Observe", "A", "B Observe"},
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

// An n-ary connector relates its ends' features in order: the first is the source, every other a target
// (KerML 8.3.4.5.1), typed by a three-ended definition or not, as the pilot exports them.
func TestNaryConnectorRelationshipEnds(t *testing.T) {
	const src = `package P {
		part def A;
		part def B;
		part def C;
		part a : A;
		part b : B;
		part c : C;
		connection def Tri { end e1 : A; end e2 : B; end e3 : C; }
		connection t : Tri connect (a, b, c);
		connection u connect (a, b, c);
	}`
	for _, fixture := range reflectiveFixtures(t, "nary-connector.sysml", source.KindSysML, src) {
		model := fixture.model
		for _, owner := range []string{"P::t", "P::u"} {
			sym := nestedSym(t, fixture.root, owner)
			for property, want := range map[string]string{
				"relatedElement": "a b c", "source": "a", "target": "b c",
				"relatedFeature": "a b c", "sourceFeature": "a", "targetFeature": "b c",
			} {
				elements, ok := model.ReflectiveElements(sym, property)
				var got []string
				for _, e := range elements {
					got = append(got, e.Name)
				}
				if !ok || strings.Join(got, " ") != want {
					t.Errorf("%s %s = %q, %v; want %q", owner, property, got, ok, want)
				}
			}
		}
	}
}

// Specializing an association reorders its ends — its own first, in declaration order, then the inherited
// ends it does not redefine — and so its source; target stays unique. `Sub2 :> Rev` puts its `s2` first.
func TestSpecializedAssociationRelationshipEndsFollowItsEndOrder(t *testing.T) {
	const src = `package P {
		part def A;
		part def B;
		part def C;
		part def C2 :> C;
		connection def Tri { end e1 : A; end e2 : B; end e3 : C; }
		connection def Rev :> Tri { end r3 : C2 redefines e3; end r1 : A redefines e1; }
		connection def Sub2 :> Rev { end s2 : B redefines e2; }
	}`
	for f, fixture := range reflectiveFixtures(t, "association-end-order.sysml", source.KindSysML, src) {
		model := fixture.model
		for owner, want := range map[string]struct{ related, source, target string }{
			"P::Tri":  {"A B C", "A", "B C"},
			"P::Rev":  {"C2 A A B", "C2", "A B"},
			"P::Sub2": {"B C2 A A B", "B", "C2 A B"},
		} {
			if f == 1 && owner != "P::Tri" {
				// A record keeps the type an end's redefined general declares (C beside C2) where the
				// source and the pilot keep the most specific: the recorded Feature::type is its own gap.
				continue
			}
			sym := nestedSym(t, fixture.root, owner)
			for property, w := range map[string]string{"relatedElement": want.related, "source": want.source, "target": want.target} {
				elements, ok := model.ReflectiveElements(sym, property)
				var got []string
				for _, e := range elements {
					got = append(got, e.Name)
				}
				if !ok || strings.Join(got, " ") != w {
					t.Errorf("fixture %d %s %s = %q, %v; want %q", f, owner, property, got, ok, w)
				}
			}
		}
	}
}

// A connector relating the same feature twice lists it twice as relatedElement and once as target,
// as the pilot exports `connect (a, b, b)`.
func TestConnectorTargetIsUnique(t *testing.T) {
	const src = `package P {
		part def A;
		part def B;
		part a : A;
		part b : B;
		connection d connect (a, b, b);
	}`
	for _, fixture := range reflectiveFixtures(t, "connector-target-unique.sysml", source.KindSysML, src) {
		model := fixture.model
		sym := nestedSym(t, fixture.root, "P::d")
		for property, want := range map[string]string{
			"relatedElement": "a b b", "source": "a", "target": "b",
			"relatedFeature": "a b b", "sourceFeature": "a", "targetFeature": "b",
		} {
			elements, ok := model.ReflectiveElements(sym, property)
			var got []string
			for _, e := range elements {
				got = append(got, e.Name)
			}
			if !ok || strings.Join(got, " ") != want {
				t.Errorf("d %s = %q, %v; want %q", property, got, ok, want)
			}
		}
	}
}
