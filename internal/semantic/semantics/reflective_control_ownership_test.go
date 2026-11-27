package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Every reader of the features a control action owns answers its parameters, each once: one the
// action declares, as an accept's payload, is no second element.
func TestControlParametersAreOwnedByTheirAction(t *testing.T) {
	m, root := buildModelWithStdlib(t, controlParameterSource)
	for _, name := range []string{"asg", "lp", "w", "lu", "cond", "once", "snd", "acc", "ter"} {
		action := nestedSym(t, root, "P::T::a::"+name)
		params := m.nodeParameters(action)
		if len(params) == 0 {
			t.Errorf("%s holds no parameters", name)
			continue
		}
		for _, feature := range []string{"ownedFeature", "ownedElement", "feature", "parameter"} {
			got, _ := m.ReflectiveElements(action, feature)
			for _, param := range params {
				if n := countOf(got, param); n != 1 {
					t.Errorf("%s %s holds %s %d times, want once", name, feature, m.nodeParameterName(action, param), n)
				}
			}
		}
	}
}

func countOf(elements []*symbols.Symbol, element *symbols.Symbol) int {
	n := 0
	for _, candidate := range elements {
		if candidate == element {
			n++
		}
	}
	return n
}
