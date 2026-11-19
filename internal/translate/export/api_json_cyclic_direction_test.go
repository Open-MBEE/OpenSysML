package export

import (
	"slices"
	"testing"
)

const cyclicDirectionModel = `package Probe {
	part def C { in attribute x : ScalarValues::Integer; out attribute y : ScalarValues::Integer; }
	part def A :> B, C;
	part def B :> A, C;
	part def P :> Q;
	part def Q :> R;
	part def R :> P, C;
}`

// Types that specialize each other still inherit a directed feature from the type that
// owns it, so reading its direction ends there instead of circling (KerML 8.3.3.1.10).
func TestAPIJSONDirectionThroughMutuallySpecializingTypes(t *testing.T) {
	graph := duplicateGraph(t, cyclicDirectionModel, IDQualifiedName)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	for _, name := range []string{"A", "B", "C", "P", "Q", "R"} {
		element := elements["Probe__"+name]
		if ids := referenceIDs(t, element, "input"); !slices.Equal(ids, []string{"Probe__C__x"}) {
			t.Errorf("%s.input = %v, want C's x", name, ids)
		}
		if ids := referenceIDs(t, element, "output"); !slices.Equal(ids, []string{"Probe__C__y"}) {
			t.Errorf("%s.output = %v, want C's y", name, ids)
		}
	}
}
