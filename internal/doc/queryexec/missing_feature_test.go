package queryexec

import (
	"errors"
	"strings"
	"testing"
)

// TestExecuteMissingFeatureIsAPathErrorUnlessTolerated: reading a feature an
// element lacks names that element and the features it has; tolerate = true
// keeps the empty cell, and only a property no row carries is then unknown.
func TestExecuteMissingFeatureIsAPathErrorUnlessTolerated(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Heavy { attribute mass : ScalarValues::Real; }
part root {
	part a : Heavy { attribute :>> mass = 1.0; }
	part b { attribute volume = 2.0; attribute mas = 3.0; }
}
calc def Masses :> Query {
	in source : Element;
	Project(source = OwnedElements(source = source), properties = ("mass"))
}
calc def ToleratedMasses :> Query {
	in source : Element;
	Project(source = OwnedElements(source = source), properties = ("mass"), tolerate = true)
}
calc def ToleratedAbsent :> Query {
	in source : Element;
	Project(source = OwnedElements(source = source), properties = ("absent"), tolerate = true)
}
calc def HeavyOnes :> Query {
	in source : Element;
	WhereFeature(source = OwnedElements(source = source), 'feature' = "mass", operator = ">", value = "0.5")
}
calc def ToleratedHeavyOnes :> Query {
	in source : Element;
	WhereFeature(source = OwnedElements(source = source), 'feature' = "mass", operator = ">", value = "0.5", tolerate = true)
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	var executionError *Error
	for _, query := range []string{"Masses", "HeavyOnes"} {
		_, err := fixture.execute(t, query, bindings, Options{})
		if !errors.As(err, &executionError) || executionError.Kind != ErrorMissingFeature {
			t.Fatalf("%s: error = %v, want missing-feature", query, err)
		}
		if executionError.Target != "Observatory::root::b" || executionError.Property != "mass" {
			t.Fatalf("%s: names %s.%s, want Observatory::root::b.mass", query, executionError.Target, executionError.Property)
		}
		if got := strings.Join(executionError.Candidates, ","); got != "mas,volume" {
			t.Fatalf("%s: frontier = %q, want the features of root::b", query, got)
		}
		if msg := executionError.Error(); !strings.HasSuffix(msg, "of Observatory::root::b, which has no such feature: its features are mas, volume") {
			t.Fatalf("%s: message = %q", query, msg)
		}
		outcome := executionError.Outcome
		if outcome == nil || outcome.Kind() != "pathError" || len(outcome.Path.Failures) != 1 {
			t.Fatalf("%s: outcome = %+v, want one path failure", query, outcome)
		}
		failure := outcome.Path.Failures[0]
		if failure.Step.Label() != "Feature(mass)" || failure.Expected != "a feature named mass on the element" || failure.Frontier.At != "Observatory::root::b" || failure.Frontier.Nearest != "mas" {
			t.Fatalf("%s: failure = %+v", query, failure)
		}
		if len(failure.Frontier.Features) != 2 || failure.Frontier.Features[0].Name != "mas" || failure.Frontier.Features[1].Name != "volume" {
			t.Fatalf("%s: frontier features = %+v", query, failure.Frontier.Features)
		}
	}
	tolerated, err := fixture.execute(t, "ToleratedMasses", bindings, Options{})
	if err != nil {
		t.Fatalf("ToleratedMasses: %v", err)
	}
	if len(tolerated.Rows()) != 2 || len(tolerated.Rows()[0].Cells()[0].Values()) != 1 || len(tolerated.Rows()[1].Cells()[0].Values()) != 0 {
		t.Fatalf("ToleratedMasses: rows = %+v, want a's mass and an empty cell for b", tolerated.Rows())
	}
	kept, err := fixture.execute(t, "ToleratedHeavyOnes", bindings, Options{})
	if err != nil || len(kept.Rows()) != 1 {
		t.Fatalf("ToleratedHeavyOnes: rows = %v, err = %v, want a alone", kept.Rows(), err)
	}
	_, err = fixture.execute(t, "ToleratedAbsent", bindings, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorUnknownProperty {
		t.Fatalf("ToleratedAbsent: error = %v, want unknown-property", err)
	}
}

// TestExecuteMissingFeatureOnAMemberPathStopsAtTheMemberReached: a dotted
// property whose segment names no member reports the member the path reached,
// with that member's features as the frontier, not the row's.
func TestExecuteMissingFeatureOnAMemberPathStopsAtTheMemberReached(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part root {
	part x { part stat { attribute count = 1; attribute runs = 2; } }
	part y { part stat { attribute count = 3; attribute run = 4; } }
}
calc def Runs :> Query {
	in source : Element;
	Project(source = OwnedElements(source = source), properties = ("stat.runs"))
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	var executionError *Error
	_, err := fixture.execute(t, "Runs", bindings, Options{})
	if !errors.As(err, &executionError) || executionError.Kind != ErrorMissingFeature {
		t.Fatalf("error = %v, want missing-feature", err)
	}
	if executionError.Target != "Observatory::root::y" || executionError.Actual != "Observatory::root::y::stat" {
		t.Fatalf("names %s stopping at %s, want root::y stopping at root::y::stat", executionError.Target, executionError.Actual)
	}
	if msg := executionError.Error(); !strings.HasSuffix(msg, "reads feature stat.runs of Observatory::root::y, but the path stops at Observatory::root::y::stat: its features are count, run") {
		t.Fatalf("message = %q", msg)
	}
	failure := executionError.Outcome.Path.Failures[0]
	if failure.Step.Label() != "Feature(runs)" || failure.Frontier.At != "Observatory::root::y::stat" || failure.Frontier.Nearest != "run" {
		t.Fatalf("failure = %+v", failure)
	}
	if len(failure.Frontier.Features) != 2 || failure.Frontier.Features[0].Name != "count" || failure.Frontier.Features[1].Name != "run" {
		t.Fatalf("frontier features = %+v", failure.Frontier.Features)
	}
}
