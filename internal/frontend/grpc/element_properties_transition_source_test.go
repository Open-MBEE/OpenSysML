package grpc

import (
	"slices"
	"testing"
)

// A target transition written without a source leaves the closest previous state usage (SysML 7.18.3),
// and its succession ends there and at the target (deriveTransitionUsageTarget), trigger or not.
func TestGetElementPropertiesEndsATriggeredTargetTransition(t *testing.T) {
	r := parseForElementProperties(t, `package Q {
		attribute def Sig;
		state def SD {
			entry; then a;
			state a;
				accept Sig
				then b;
			state b;
			transition t2 first b then a;
			state c { }
				accept Sig if true then a;
		}
	}`)
	for _, c := range []struct{ transition, source, target string }{
		{"Q__SD___403", "Q::SD::a", "Q::SD::b"},
		{"Q__SD__t2", "Q::SD::b", "Q::SD::a"},
		{"Q__SD___407", "Q::SD::c", "Q::SD::a"},
	} {
		for property, want := range map[string]string{"source": c.source, "target": c.target} {
			if got := answeredElements(t, r, c.transition, property); !slices.Equal(got, []string{want}) {
				t.Errorf("%s.%s = %v, want %s", c.transition, property, got, want)
			}
		}
		succession := c.transition + "_succession"
		for property, want := range map[string][]string{
			"sourceFeature":  {c.source},
			"targetFeature":  {c.target},
			"relatedFeature": {c.source, c.target},
		} {
			if got := answeredElements(t, r, succession, property); !slices.Equal(got, want) {
				t.Errorf("%s.%s = %v, want %v", succession, property, got, want)
			}
		}
	}
}
