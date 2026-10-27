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
