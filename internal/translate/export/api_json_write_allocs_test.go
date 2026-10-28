package export

import (
	"fmt"
	"strings"
	"testing"
)

// The writer reads each element's owners, positions and types through one index
// built per write; each further part definition and usage stays under a fixed cost.
func TestWriteAPIJSONAllocationsPerElement(t *testing.T) {
	perElement := 2720.0
	if raceBuild {
		perElement = 3140
	}
	allocs := func(n int) float64 {
		var src strings.Builder
		src.WriteString("package P {\n")
		for i := range n {
			fmt.Fprintf(&src, "\tpart def A%d { attribute a%d; attribute b; }\n\tpart p%d : A%d;\n", i, i, i, i)
		}
		src.WriteString("}\n")
		graph := normativeGraph(t, src.String())
		return testing.AllocsPerRun(3, func() {
			if _, err := WriteAPIJSON(graph); err != nil {
				t.Fatal(err)
			}
		})
	}
	const n = 100
	single, double := allocs(n), allocs(2*n)
	if per := (double - single) / n; per > perElement {
		t.Errorf("writing %d part definitions and usages allocates %.0f, %d allocates %.0f: %.0f per element; want at most %.0f",
			n, single, 2*n, double, per, perElement)
	}
}
