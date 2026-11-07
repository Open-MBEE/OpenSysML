package runtime

import (
	"slices"
	"testing"
)

const stateUsageParameterValuesSource = `package P {
	state def M {
		in n;
		entry; then s1;
		state s1;
		state s2;
		transition first s1 if n > 2 then s2;
	}
	state m : M {
		in n = 3;
	}
	state r : M {
		in n redefines n = 3;
	}
	state def D {
		in n default 3;
		entry; then s1;
		state s1;
		state s2;
		transition first s1 if n > 2 then s2;
	}
	state d : D {
		in n;
	}
	state def Count {
		attribute n = 1;
		entry; then s1;
		state s1;
		state s2;
		transition first s1 if n > 2 then s2;
	}
	state hides : Count {
		attribute n = 9;
	}
}
`

func stateUsageVisits(t *testing.T, name string) []string {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, stateUsageParameterValuesSource))
	_, visits, err := ctx.ExecuteStateWithEvents(oneSymbol(t, idx, "P::"+name), nil)
	if err != nil {
		t.Fatalf("ExecuteState(%s): %v", name, err)
	}
	return visits
}

// A state usage run with no argument reads its own `in` value where the
// definition's guard names the parameter, redefined implicitly by name or explicitly.
func TestStateUsageOwnInputValueIsRead(t *testing.T) {
	for _, name := range []string{"m", "r"} {
		if visits := stateUsageVisits(t, name); !slices.Equal(visits, []string{"s1", "s2"}) {
			t.Errorf("%s visited %v, want [s1 s2]", name, visits)
		}
	}
}

// A usage parameter stating no value keeps the default of the one it redefines.
func TestStateUsageUnvaluedParameterKeepsTheDefault(t *testing.T) {
	if visits := stateUsageVisits(t, "d"); !slices.Equal(visits, []string{"s1", "s2"}) {
		t.Errorf("d visited %v, want [s1 s2]", visits)
	}
}

// A usage member that only hides a same-named attribute of the definition,
// redefining nothing, leaves the guard reading the definition's own (KerML 1.0 §8.2.3.5.3).
func TestStateUsageNamesakeDoesNotReplaceGuardFeature(t *testing.T) {
	if visits := stateUsageVisits(t, "hides"); !slices.Equal(visits, []string{"s1"}) {
		t.Errorf("hides visited %v, want [s1]", visits)
	}
}
