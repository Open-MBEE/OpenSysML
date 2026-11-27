package export_test

import (
	"slices"
	"testing"
)

// The element form ends a transition written after a state without a source at the closest state usage
// before it (SysML v2 7.18.3), as the reader does, and its succession at that state and the target.
func TestAPIJSONTriggeredTargetTransitionLeavesThePreviousState(t *testing.T) {
	elements := apiJSONOf(t, `package Q {
	attribute def Sig;
	state def SD {
		entry; then a;
		state a;
		accept Sig then b;
		state b;
		transition t2 first b then a;
		state c { entry action warm; }
		accept Sig then a;
	}
}`)
	for _, c := range []struct{ transition, source, target string }{
		{"Q__SD___403", "Q__SD__a", "Q__SD__b"},
		{"Q__SD__t2", "Q__SD__b", "Q__SD__a"},
		// A state usage declaring a body is left as one declaring none is.
		{"Q__SD___407", "Q__SD__c", "Q__SD__a"},
	} {
		transition := elements[c.transition]
		if got := refIDs(transition["source"]); !slices.Equal(got, []string{c.source}) {
			t.Errorf("%s source = %v, want %s", c.transition, got, c.source)
		}
		if got := refIDs(transition["target"]); !slices.Equal(got, []string{c.target}) {
			t.Errorf("%s target = %v, want %s", c.transition, got, c.target)
		}
		succession := elements[c.transition+"_succession"]
		if got := refIDs(succession["sourceFeature"]); !slices.Equal(got, []string{c.source}) {
			t.Errorf("%s succession sourceFeature = %v, want %s", c.transition, got, c.source)
		}
		if got := refIDs(succession["targetFeature"]); !slices.Equal(got, []string{c.target}) {
			t.Errorf("%s succession targetFeature = %v, want %s", c.transition, got, c.target)
		}
		if got := refIDs(succession["relatedFeature"]); !slices.Equal(got, []string{c.source, c.target}) {
			t.Errorf("%s succession relatedFeature = %v, want %s then %s", c.transition, got, c.source, c.target)
		}
	}
}
