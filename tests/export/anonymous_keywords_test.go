package export_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// anonymousFixture converts one testdata/convert fixture to Turtle.
func anonymousFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "convert", name)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	format, err := export.FormatOfPath(path)
	if err != nil {
		t.Fatal(err)
	}
	turtle, err := export.Convert(path, src, format, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	return turtle
}

// TestAnonymousKeywordsAreTypedWhereTheVocabularyStatesThem pins which fact carries
// each anonymous keyword: a typed one where the vocabulary has it, else the spelling.
func TestAnonymousKeywordsAreTypedWhereTheVocabularyStatesThem(t *testing.T) {
	cases := []struct {
		fixture       string
		want, forbade []string
	}{{
		fixture: "anonymous_events.sysml",
		want: []string{
			"a sysml:EventOccurrenceUsage ;",
			"sysx:declaredKeyword \"event\" ;\n    sysml:references \"exchange.request\"^^sysx:Expression ;",
		},
		forbade: []string{"sysml:isEvent", "a sysml:OccurrenceUsage ;\n    sysml:qualifiedName \"AnonymousEvents::Sender::@"},
	}, {
		fixture: "anonymous_portions.sysml",
		want: []string{
			"sysx:declaredKeyword \"snapshot\" ;\n    sysml:isPortion \"true\"^^xsd:boolean ;\n    sysml:portionKind \"snapshot\" ;\n    sysml:redefines \"start\" ;",
			"sysx:declaredKeyword \"timeslice\" ;\n    sysml:isPortion \"true\"^^xsd:boolean ;\n    sysml:portionKind \"timeslice\" ;\n    sysml:redefines \"portionOfLife\" ;",
		},
		forbade: []string{"sysml:isSnapshot", "sysml:isTimeslice"},
	}, {
		fixture: "anonymous_assertions.sysml",
		want: []string{
			"a sysml:AssertConstraintUsage ;",
			"sysx:declaredKeyword \"assert\" ;\n    sysml:references elmt:AnonymousAssertions__massConstraint ;",
			"sysml:isNegated \"true\"^^xsd:boolean ;\n    sysml:references elmt:AnonymousAssertions__massConstraint ;",
		},
		forbade: []string{"sysx:declaredPrefix \"assert\""},
	}, {
		fixture: "anonymous_features.kerml",
		want: []string{
			"sysx:declaredKeyword \"feature\" ;\n    sysml:redefines elmt:AnonymousFeatures__Part__m ;",
			"sysx:declaredKeyword \"feature\" ;\n    sysml:redefines elmt:AnonymousFeatures__Part__accessedFeature ;",
		},
	}}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			turtle := string(anonymousFixture(t, tc.fixture))
			for _, want := range tc.want {
				if !strings.Contains(turtle, want) {
					t.Errorf("the graph should state\n%s\n--- graph ---\n%s", want, turtle)
				}
			}
			for _, forbade := range tc.forbade {
				if strings.Contains(turtle, forbade) {
					t.Errorf("the graph should not state %q\n--- graph ---\n%s", forbade, turtle)
				}
			}
		})
	}
}

// TestAnonymousKeywordsComeBackFromTheTypedFactsAlone strips the spelling too: the
// typed facts spell the long form (or canonical `attribute`) and the graph holds.
func TestAnonymousKeywordsComeBackFromTheTypedFactsAlone(t *testing.T) {
	cases := []struct {
		fixture string
		want    []string
	}{{
		fixture: "anonymous_events.sysml",
		want:    []string{"event occurrence", "event occurrence ack;", "event occurrence : Exchange;"},
	}, {
		fixture: "anonymous_portions.sysml",
		want:    []string{"snapshot occurrence redefines start {", "timeslice occurrence redefines portionOfLife {", "snapshot occurrence subsets fuel redefines done {"},
	}, {
		fixture: "anonymous_assertions.sysml",
		want:    []string{"assert constraint references massConstraint {", "assert not constraint references massConstraint;", "assert constraint {"},
	}, {
		fixture: "anonymous_features.kerml",
		want:    []string{"attribute redefines m {", "attribute redefines accessedFeature;", "inout attribute redefines accessedFeature, m;"},
	}}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			name, _ := fixtureName(tc.fixture)
			first := anonymousFixture(t, tc.fixture)
			typedOnly := withoutTriples(t, withoutSourceText(t, first), "sysx:declaredKeyword")
			back, err := export.Convert(name+".ttl", typedOnly, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation from the typed facts alone: %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(string(back), want) {
					t.Errorf("the typed facts should spell %q\n--- notation ---\n%s", want, back)
				}
			}
			again, err := export.Convert(name+".sysml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if lost, gained := tripleSetDiff(t, typedOnly, withoutTriples(t, withoutSourceText(t, again), "sysx:declaredKeyword")); len(lost)+len(gained) > 0 {
				t.Errorf("the typed facts alone changed the graph\n--- notation ---\n%s\n--- lost ---\n%s\n--- gained ---\n%s",
					back, strings.Join(lost, "\n"), strings.Join(gained, "\n"))
			}
		})
	}
}

// TestContradictoryKeywordGraphsAreRefused corrupts the typed fact under an anonymous
// keyword: the decoder refuses rather than spelling the canonical keyword.
func TestContradictoryKeywordGraphsAreRefused(t *testing.T) {
	snapshot := "    sysx:declaredKeyword \"snapshot\" ;\n    sysml:isPortion \"true\"^^xsd:boolean ;\n    sysml:portionKind \"snapshot\" ;\n    sysml:redefines \"start\" ;"
	event := "    a sysml:EventOccurrenceUsage ;\n    sysml:qualifiedName \"AnonymousEvents::Sender::@1\" ;"
	asserted := "    a sysml:AssertConstraintUsage ;\n    sysml:qualifiedName \"AnonymousAssertions::Vehicle::@2\" ;"
	cases := []struct {
		name, fixture, old, new string
		want                    []string
	}{{
		name: "portion_missing", fixture: "anonymous_portions.sysml",
		old: snapshot, new: "    sysx:declaredKeyword \"snapshot\" ;\n    sysml:isPortion \"true\"^^xsd:boolean ;\n    sysml:redefines \"start\" ;",
		want: []string{"the `snapshot` declaration <urn:sysmlv2:element:AnonymousPortions__car___400>", "no sysml:portionKind"},
	}, {
		name: "portion_wrong", fixture: "anonymous_portions.sysml",
		old: snapshot, new: strings.Replace(snapshot, "portionKind \"snapshot\"", "portionKind \"timeslice\"", 1),
		want: []string{"the `snapshot` declaration", `sysml:portionKind "timeslice", not the "snapshot" its keyword states`},
	}, {
		name: "portion_unknown", fixture: "anonymous_portions.sysml",
		old: snapshot, new: strings.Replace(snapshot, "portionKind \"snapshot\"", "portionKind \"slice\"", 1),
		want: []string{`the portion kind "slice" of <urn:sysmlv2:element:AnonymousPortions__car___400>`, "is `snapshot` or `timeslice`"},
	}, {
		name: "event_untyped", fixture: "anonymous_events.sysml",
		old: event, new: strings.Replace(event, "EventOccurrenceUsage", "OccurrenceUsage", 1),
		want: []string{"the `event` declaration <urn:sysmlv2:element:AnonymousEvents__Sender___401>", "the metaclass OccurrenceUsage, not the EventOccurrenceUsage its keyword states"},
	}, {
		name: "assert_untyped", fixture: "anonymous_assertions.sysml",
		old: asserted, new: strings.Replace(asserted, "AssertConstraintUsage", "ConstraintUsage", 1),
		want: []string{"the `assert` declaration <urn:sysmlv2:element:AnonymousAssertions__Vehicle___402>", "the metaclass ConstraintUsage, not the AssertConstraintUsage its keyword states"},
	}, {
		name: "event_named", fixture: "anonymous_events.sysml",
		old: event, new: event + "\n    sysml:declaredName \"e\" ;",
		want: []string{"the `event` declaration <urn:sysmlv2:element:AnonymousEvents__Sender___401>", "it declares a name (sysml:declaredName), which `event` written as the kind keyword cannot", "a declaration is written `event occurrence <name>`"},
	}, {
		name: "event_named_unreferenced", fixture: "anonymous_events.sysml",
		old:  "    sysx:declaredKeyword \"event\" ;\n    sysml:references \"exchange.request\"^^sysx:Expression ;",
		new:  "    sysx:declaredKeyword \"event\" ;\n    sysml:declaredName \"e\" ;",
		want: []string{"the `event` declaration <urn:sysmlv2:element:AnonymousEvents__Sender___401>", "would come back as a reference to a different element"},
	}, {
		name: "assert_named_body", fixture: "anonymous_assertions.sysml",
		old:  "    sysx:declaredKeyword \"assert\" ;\n    sysml:references elmt:AnonymousAssertions__massConstraint ;",
		new:  "    sysx:declaredKeyword \"assert\" ;\n    sysml:declaredName \"c2\" ;",
		want: []string{"the `assert` declaration <urn:sysmlv2:element:AnonymousAssertions__Vehicle___402>", "nothing but a body or a multiplicity follows", "a declaration is written `assert constraint <name>`"},
	}, {
		name: "assert_named_multiplicity", fixture: "anonymous_assertions.sysml",
		old:  "    sysx:declaredKeyword \"assert\" ;\n    sysml:references elmt:AnonymousAssertions__massConstraint ;",
		new:  "    sysx:declaredKeyword \"assert\" ;\n    sysml:declaredName \"c2\" ;\n    sysml:upperBound \"1\"^^xsd:integer ;\n    sysml:references elmt:AnonymousAssertions__massConstraint ;",
		want: []string{"the `assert` declaration <urn:sysmlv2:element:AnonymousAssertions__Vehicle___402>", "nothing but a body or a multiplicity follows"},
	}, {
		name: "assert_other_prefix", fixture: "anonymous_assertions.sysml",
		old: asserted, new: asserted + "\n    sysx:declaredPrefix \"assume\" ;",
		want: []string{"the asserted constraint <urn:sysmlv2:element:AnonymousAssertions__Vehicle___402>", `sysx:declaredPrefix "assume" is not the ` + "`assert` its metaclass states"},
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			turtle := editTurtle(t, withoutSourceText(t, anonymousFixture(t, tc.fixture)), tc.old, tc.new)
			_, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("expected an UnsupportedError, got %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected %q in error:\n%s", want, err)
				}
			}
		})
	}
}
