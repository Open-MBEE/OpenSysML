package view

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRepeatedExpansionFoldDepthDominance(t *testing.T) {
	for _, tc := range []struct{ depth, padding int }{{3, 0}, {maxTreeDepth, 10000}} {
		t.Run(fmt.Sprintf("depth%d-padding%d", tc.depth, tc.padding), func(t *testing.T) {
			checkExpansionFoldDepth(t, tc.depth, tc.padding)
		})
	}
}

func checkExpansionFoldDepth(t *testing.T, depth, padding int) {
	t.Helper()
	var source strings.Builder
	source.WriteString(`package P {
	private import Views::*;
	part def Shared { part missing; }
	part def Outer { part repeated : Shared; }
	part def Carrier { part outer : Outer; }
	part padding {`)
	for i := range padding {
		fmt.Fprintf(&source, "part filler%d;\n", i)
	}
	source.WriteString("} part deep {")
	for i := range depth - 3 {
		fmt.Fprintf(&source, "part layer%d {", i)
	}
	source.WriteString("part extra : Carrier;")
	for range depth - 3 {
		source.WriteString("}")
	}
	source.WriteString(`} part shallow : Carrier;
	view v { expose padding; expose deep; expose shallow; render asTreeDiagram; } }`)
	r, idx := loadSources(t, []string{"fold-depth.sysml"}, [][]byte{[]byte(source.String())})
	if depth != maxTreeDepth {
		r.SetDepth(depth)
	}
	v := lookup(t, idx, "P::v")
	missing := lookup(t, idx, "P::Shared::missing")
	query, err := r.DrawnIn(v)
	if err != nil {
		t.Fatal(err)
	}
	r.expansionFoldThreshold = -1
	full, err := r.Render(v)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(full.Drawn(), query) || !query.Node(missing) {
		t.Fatal("unfolded baseline must match query and draw missing")
	}
	var count func([]*Node) int
	count = func(nodes []*Node) int {
		n := len(nodes)
		for _, node := range nodes {
			n += count(node.Children)
		}
		return n
	}
	if padding > 0 && count(full.Roots) != 10022 {
		t.Fatalf("default-depth regression has %d nodes; want 10022", count(full.Roots))
	}
	t.Logf("unfolded nodes=%d depth bound=%d", count(full.Roots), r.walkBound(v))
	ids := map[string]bool{}
	var find func([]*Node)
	find = func(nodes []*Node) {
		for _, node := range nodes {
			if node.Origin == symbolOrigin(missing) {
				ids[node.ID] = true
				if node.Geometry != nil {
					t.Fatal("counterexample member must not be positioned")
				}
			}
			find(node.Children)
		}
	}
	find(full.Roots)
	if len(ids) == 0 {
		t.Fatal("baseline must contain missing node")
	}
	for _, edge := range full.Edges {
		if ids[edge.From] || ids[edge.To] {
			t.Fatal("counterexample member must not be an edge endpoint")
		}
	}
	r.expansionFoldThreshold = 0
	if padding == 0 {
		r.expansionFoldThreshold = 1
	}
	folded, err := r.Render(v)
	if err != nil {
		t.Fatal(err)
	}
	if !folded.expansionFolding {
		t.Fatal("regression must activate folding")
	}
	if !folded.Drawn().Node(missing) {
		t.Fatal("fold lost inherited member reached with more remaining depth")
	}
	if !reflect.DeepEqual(folded.Drawn(), query) {
		t.Fatal("fold must preserve the complete drawn set")
	}
}

func TestRepeatedExpansionFoldNamesDeepestRepresentative(t *testing.T) {
	src := `package P {
		private import Views::*;
		part def Shared { part missing; }
		part def Outer { part repeated : Shared; }
		part def Carrier { part outer : Outer; }
		part earlier : Carrier;
		part later { part extra : Carrier; }
		view v { expose earlier; expose later; render asTreeDiagram; }
	}`
	r, idx := loadSources(t, []string{"dominance.sysml"}, [][]byte{[]byte(src)})
	r.expansionFoldThreshold = 1
	r.SetDepth(4)
	v := lookup(t, idx, "P::v")
	full, err := r.Render(v)
	if err != nil {
		t.Fatal(err)
	}
	query, err := r.DrawnIn(v)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(full.Drawn(), query) {
		t.Fatal("fold changed drawn set")
	}
	want := "features of P::Shared shown at P::earlier::outer::repeated; repeated expansion folded"
	for _, node := range everyNode(full.Roots) {
		if node.Detail == want {
			return
		}
	}
	t.Fatalf("missing representative note %q", want)
}
