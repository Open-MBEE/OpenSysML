package export

import (
	"fmt"
	"strings"
	"testing"
)

// The evaluator reads names and related elements through the graph structure
// once per element; a read of a stated scalar or reference must not allocate.
func TestAPIJSONGraphStructureReadsDoNotAllocate(t *testing.T) {
	var src strings.Builder
	src.WriteString("package P {\n")
	for i := range 50 {
		fmt.Fprintf(&src, "\tpart def A%d;\n", i)
	}
	src.WriteString("}\n")
	structure := newAPIJSONEvaluation(normativeGraph(t, src.String())).structure
	reads := 0
	for _, element := range structure.elements {
		if class, ok := structure.Metaclass(element); !ok || class != "PartDefinition" {
			continue
		}
		reads++
		for _, read := range []struct {
			name string
			run  func()
		}{
			{"Attribute name", func() { structure.Attribute(element, "name") }},
			{"StatedAttribute name", func() { structure.StatedAttribute(element, "name") }},
			{"HasUnresolvedRelatedElement owningRelationship", func() {
				structure.HasUnresolvedRelatedElement(element, "owningRelationship")
			}},
		} {
			if allocs := testing.AllocsPerRun(10, read.run); allocs != 0 {
				t.Fatalf("%s of %s allocates %.0f times per read; want none", read.name, element.Aspect, allocs)
			}
		}
	}
	if reads != 50 {
		t.Fatalf("read %d part definitions; want 50", reads)
	}
}
