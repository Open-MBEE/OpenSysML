package export_test

import (
	"strings"
	"testing"
)

const satisfyReferenceModel = `package P {
	requirement def R;
	requirement r : R;
	requirement other : R;
	requirement group { requirement nested : R; }
	part v;
	part context {
		satisfy r :> other by v;
		satisfy group.nested by v;
		satisfy requirement declared :> r by v;
	}
	verification def V {
		objective {
			verify r :> other;
		}
	}
}
`

// TestSatisfyReferenceIsTheOneReferenceSubsetting pins SysML v2 §8.2.2.21.2 and KerML §8.3.3.3.9: the
// requirement named is the usage's one ReferenceSubsetting, a specialization after it a Subsetting.
func TestSatisfyReferenceIsTheOneReferenceSubsetting(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "p.sysml", []byte(satisfyReferenceModel))
	graph := string(turtle)
	if got := strings.Count(graph, "a sysml:ReferenceSubsetting ;"); got != 3 {
		t.Errorf("each reference form owns one ReferenceSubsetting, 3 in all; the graph states %d:\n%s", got, graph)
	}
	for _, want := range []string{
		"    sysml:subsets elmt:P__other ;\n    sysml:references elmt:P__r ;",
		"    sysml:subsets elmt:P__r ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	notation := string(back)
	for _, want := range []string{
		"satisfy r subsets other by v;\n",
		"satisfy group.nested by v;\n",
		"satisfy requirement declared subsets r by v;\n",
		"verify r subsets other;\n",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
}
