package view

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func wideQueryModel(width int) string {
	var src strings.Builder
	src.WriteString("package P { private import Views::*; part root {\n")
	for member := range width {
		fmt.Fprintf(&src, "part p%d;\n", member)
	}
	src.WriteString("} view v { expose root; render asTreeDiagram; } }")
	return src.String()
}

func TestTreeDrawnQueryShallowAllocations(t *testing.T) {
	allocated := map[int]int64{}
	for _, width := range []int{10000, 20000, 40000} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			r, idx := loadSources(t, []string{"wide.sysml"}, [][]byte{[]byte(wideQueryModel(width))})
			r.SetDepth(1)
			view := lookup(t, idx, "P::v")
			drawn, err := r.DrawnIn(view)
			if err != nil {
				t.Fatal(err)
			}
			full, err := r.Render(view)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(full.Drawn(), drawn) {
				t.Fatal("depth-one query differs from full render")
			}
			if !drawn.Node(lookup(t, idx, fmt.Sprintf("P::root::p%d", width-1))) {
				t.Fatal("last member is not drawn")
			}
			roots := []*symbols.Symbol{lookup(t, idx, "P::root")}
			// Isolate graph preparation from the renderer's linear work; never reuse a prepared graph.
			result := testing.Benchmark(func(b *testing.B) {
				for range b.N {
					fresh := NewRenderer(r.model, r.resolver, r.text)
					fresh.queryMembers = r.queryMembers
					fresh.SetDepth(1)
					fresh.treeQuery(view, &Rendering{query: true}, roots)
				}
			})
			allocated[width] = result.AllocedBytesPerOp()
			t.Logf("depth-one graph preparation: %d B/op", allocated[width])
		})
	}
	// About twice the 40k preparation budget, below the old dense table alone.
	const ceiling = 112 << 20
	if allocated[40000] > ceiling {
		t.Errorf("graph preparation allocated %d bytes, exceeding %d", allocated[40000], ceiling)
	}
	if allocated[40000]*2 > allocated[20000]*5 {
		t.Errorf("graph allocations grew by more than 2.5x: %d -> %d", allocated[20000], allocated[40000])
	}
}
