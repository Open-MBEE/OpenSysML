package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// memberFeatureModel is the shape that first lost a triple set on the graph-only
// round trip: `member` features and a `featured by` naming an anonymous portion.
const memberFeatureModel = `package T {
    class CC1 {
        member feature x featured by CC1_snapshots {
            member feature CC1_snapshots :>> Occurrences::Occurrence::snapshots featured by CC1;
        }
        portion :>> startShot {
            member feature :>> CC1::x featured by CC1_startShot_snapshots = 0 {
                member feature CC1_startShot_snapshots :>> CC1_snapshots featured by CC1::startShot;
            }
        }
    }
    class Plain {
        feature p;
    }
}
`

// withoutLayout strips the two predicates that carry layout alone, keeping
// sysx:sourceLanguage: the grammar a model is written in is a fact about it.
func withoutLayout(t *testing.T, turtle []byte) []byte {
	t.Helper()
	for _, property := range []string{"sysx:sourceText", "sysx:sourceTail"} {
		turtle = withoutTriples(t, turtle, property)
	}
	return turtle
}

// graphOnlyRoundTrip converts notation to Turtle, strips the layout, writes the
// notation back and converts that again, failing unless the stripped graphs agree.
func graphOnlyRoundTrip(t *testing.T, name string, src []byte) (turtle, back []byte) {
	t.Helper()
	turtle, err := convert.Convert(name, src, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err = convert.Convert("m.ttl", withoutLayout(t, turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the graph alone: %v", err)
	}
	again, err := convert.Convert(name, back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v\n%s", err, back)
	}
	if lost, gained := tripleSetDiff(t, withoutLayout(t, turtle), withoutLayout(t, again)); len(lost)+len(gained) > 0 {
		t.Errorf("the graph-only hop changed the graph\n--- notation ---\n%s\n--- lost ---\n%s\n--- gained ---\n%s",
			back, strings.Join(lost, "\n"), strings.Join(gained, "\n"))
	}
	return turtle, back
}

// A KerML `member feature` is a feature its type owns through a plain
// OwningMembership (KerML.xtext TypeFeatureMember), not a FeatureMembership, and
// comes back as `member`; an ordinary `feature` keeps its FeatureMembership.
func TestMemberFeatureIsAnOwningMembership(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "m.kerml", []byte(memberFeatureModel))
	graph := string(turtle)
	for _, want := range []string{
		"elmt:T__CC1__x_om\n    a sysml:OwningMembership ;",
		"elmt:T__CC1__x__CC1_5fsnapshots_om\n    a sysml:OwningMembership ;",
		"elmt:T__Plain__p_om\n    a sysml:FeatureMembership ;",
		"sysml:ownedMemberFeature elmt:T__Plain__p ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	for _, unwanted := range []string{
		"sysml:ownedMemberFeature elmt:T__CC1__x ;",
		"sysml:ownedFeature elmt:T__CC1__x ;",
	} {
		if strings.Contains(graph, unwanted) {
			t.Errorf("a `member` feature is not a feature of its type, yet the graph states %q:\n%s", unwanted, graph)
		}
	}
	notation := string(back)
	for _, want := range []string{
		"        member feature x featured by CC1_snapshots {\n",
		"            member feature CC1_snapshots redefines snapshots featured by CC1;\n",
		"        portion redefines startShot {\n",
		"            member feature redefines x featured by CC1_startShot_snapshots = 0 {\n",
		"        feature p;\n",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
	if strings.Contains(notation, "member feature p") {
		t.Errorf("an ordinary feature came back as `member`:\n%s", notation)
	}
}

// The `featured by` inside the anonymous portion names that portion, which the
// inherited `startShot` shadows there, so it is written qualified and never as a
// literal in place of the IRI the first graph held.
func TestFeaturedByAnonymousPortionResolvesWhereWritten(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "m.kerml", []byte(memberFeatureModel))
	if !strings.Contains(string(back), "member feature CC1_startShot_snapshots redefines CC1_snapshots featured by CC1::startShot;\n") {
		t.Errorf("the reference to the anonymous portion should be written qualified:\n%s", back)
	}
	again, err := convert.Convert("m.kerml", back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	for _, graph := range []string{string(turtle), string(again)} {
		if strings.Contains(graph, "sysml:featuringType \"") {
			t.Errorf("a featuring type is an element, not a literal:\n%s", graph)
		}
		if !strings.Contains(graph, "sysml:featuringType elmt:T__CC1___401 ;") {
			t.Errorf("the graph should feature CC1_startShot_snapshots by the anonymous portion:\n%s", graph)
		}
	}
}

// With its source text the model comes back byte for byte, as before.
func TestMemberFeatureSourceTextRoundTripsUnchanged(t *testing.T) {
	turtle, err := convert.Convert("m.kerml", []byte(memberFeatureModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := convert.Convert("m.ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if string(back) != memberFeatureModel {
		t.Errorf("the notation changed\n--- want ---\n%s\n--- got ---\n%s", memberFeatureModel, back)
	}
}

// A `member` feature carries the visibility, the prefix `member` follows.
func TestMemberFeatureKeepsItsVisibility(t *testing.T) {
	src := "package T {\n    class C {\n        private member feature x;\n        protected member feature y :>> x;\n    }\n}\n"
	_, back := graphOnlyRoundTrip(t, "m.kerml", []byte(src))
	for _, want := range []string{"        private member feature x;\n", "        protected member feature y redefines x;\n"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the notation should contain %q:\n%s", want, back)
		}
	}
}

// An end's cross feature is written in its head when it was declared there and
// as a body `member feature` when it was declared with the keyword; each form
// comes back from the graph alone as the form it was.
func TestCrossFeatureFormsComeBackFromTheGraphAlone(t *testing.T) {
	src := `package P {
    class Cart;
    class Product;
    assoc Head {
        end inCart[0..1] feature cart : Cart[1];
        end feature product : Product[1];
    }
    assoc Body {
        end feature cart : Cart[1] {
            member feature inCart[0..1];
        }
        end feature product : Product[1];
    }
}
`
	turtle, back := graphOnlyRoundTrip(t, "m.kerml", []byte(src))
	for _, want := range []string{
		"        end inCart[0..1] feature cart : Cart[1];\n",
		"        end feature cart : Cart[1] {\n            member feature inCart[0..1];\n        }\n",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the notation should contain %q:\n%s", want, back)
		}
	}
	for _, want := range []string{
		"elmt:P__Head__cart__inCart\n    a sysml:Feature ;",
		"elmt:P__Head__cart__inCart_om\n    a sysml:OwningMembership ;",
		"elmt:P__Body__cart__inCart\n    a sysml:AttributeUsage ;",
		"elmt:P__Body__cart__inCart_om\n    a sysml:OwningMembership ;",
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph should state %q:\n%s", want, turtle)
		}
	}
}

// SysML has no `member` keyword: a feature a SysML type owns through a plain
// OwningMembership is refused rather than written as a feature of the type.
func TestSysMLOwningMembershipFeatureIsRefused(t *testing.T) {
	src := "package P {\n    part def Car {\n        attribute mass : Real;\n    }\n}\n"
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	foreign := strings.ReplaceAll(string(withoutLayout(t, turtle)), "sysml:FeatureMembership", "sysml:OwningMembership")
	for _, property := range []string{"sysml:ownedFeature", "sysml:ownedFeatureMembership", "sysml:ownedMemberFeature", "sysml:owningType", "json:ownedFeature", "json:ownedFeatureMembership"} {
		foreign = string(withoutTriples(t, []byte(foreign), property))
	}
	out, err := convert.Convert("m.ttl", []byte(foreign), convert.FormatTurtle, convert.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError, got %v; notation:\n%s", err, out)
	}
	for _, want := range []string{"<urn:sysmlv2:element:P__Car__mass>", "sysml:OwningMembership", "`member`", "SysML"} {
		if !strings.Contains(unsupported.Error(), want) {
			t.Errorf("the refusal should mention %q: %v", want, unsupported)
		}
	}
	if strings.Contains(string(out), "attribute mass") {
		t.Errorf("the feature was written as a feature of the type:\n%s", out)
	}
}

// The pilot's TimeVaryingFeatures.kerml, the one corpus model whose triple set
// moved on the graph-only round trip, now comes back from the graph alone.
func TestTimeVaryingFeaturesComeBackFromTheGraphAlone(t *testing.T) {
	_, src := pilotCorpusModel(t, "kerml-examples/Variable Feature Examples/TimeVaryingFeatures.kerml")
	_, back := graphOnlyRoundTrip(t, "TimeVaryingFeatures.kerml", src)
	if strings.Count(string(back), "member feature") != strings.Count(string(src), "member feature") {
		t.Errorf("the notation should keep every `member feature`:\n%s", back)
	}
}
