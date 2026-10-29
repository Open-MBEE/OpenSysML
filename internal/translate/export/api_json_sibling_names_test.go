package export

import (
	"fmt"
	"strings"
	"testing"
)

// Each member's qualified name checks the names of the members before it; that
// check must read each sibling's name once per namespace, not once per member.
func TestAPIJSONSiblingQualifiedNamesScaleLinearly(t *testing.T) {
	allocs := func(n int) float64 {
		var src strings.Builder
		src.WriteString("package P {\n")
		for i := range n {
			fmt.Fprintf(&src, "\tpart def A%d;\n", i)
		}
		src.WriteString("}\n")
		graph := normativeGraph(t, src.String())
		return testing.AllocsPerRun(2, func() {
			if _, err := apiJSONElements(graph); err != nil {
				t.Fatal(err)
			}
		})
	}
	const n = 200
	single, double := allocs(n), allocs(2*n)
	if ratio := double / single; ratio > 2.2 {
		t.Errorf("writing %d sibling members allocates %.0f, %d allocates %.0f (%.2fx); want linear growth",
			n, single, 2*n, double, ratio)
	}
}
