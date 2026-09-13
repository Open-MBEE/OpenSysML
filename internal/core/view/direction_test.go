package view

import (
	"strings"
	"testing"
)

func TestParseDirection(t *testing.T) {
	for _, name := range []string{"TB", "LR", "RL", "BT"} {
		direction, ok := ParseDirection(name)
		if !ok || string(direction) != name {
			t.Errorf("ParseDirection(%q) = %q, %v", name, direction, ok)
		}
	}
	for _, name := range []string{"", "tb", "TD", "sideways"} {
		if _, ok := ParseDirection(name); ok {
			t.Errorf("ParseDirection(%q) accepted", name)
		}
	}
}

func TestSupportsDirection(t *testing.T) {
	for _, kind := range []Kind{KindTree, KindInterconnection, KindState, KindAction} {
		if !kind.SupportsDirection() {
			t.Errorf("%s should support direction", kind)
		}
	}
	for _, kind := range []Kind{KindSequence, KindTable, KindTextual, KindGeometry} {
		if kind.SupportsDirection() {
			t.Errorf("%s should not support direction", kind)
		}
	}
}

func TestMermaidWithDirection(t *testing.T) {
	rendering := &Rendering{Kind: KindTree, Roots: []*Node{{ID: "n0", Kind: "part", Name: "a"}}}
	if got := rendering.MermaidWith(Options{Direction: DirectionBottomTop}); !strings.Contains(got, "flowchart BT") {
		t.Errorf("tree BT:\n%s", got)
	}
	if got := rendering.Mermaid(); !strings.Contains(got, "flowchart TD") {
		t.Errorf("undirected tree:\n%s", got)
	}
	states := &Rendering{Kind: KindState, Roots: []*Node{{ID: "n0", Kind: "state", Name: "s"}}}
	if got := states.MermaidWith(Options{Direction: DirectionLeftRight}); !strings.Contains(got, "direction LR") {
		t.Errorf("state LR:\n%s", got)
	}
	if got := states.Mermaid(); strings.Contains(got, "direction") {
		t.Errorf("undirected state carries a direction:\n%s", got)
	}
}

func TestRenderingClone(t *testing.T) {
	original := &Rendering{
		Kind:    KindTree,
		Roots:   []*Node{{ID: "n0", Name: "a", Children: []*Node{{ID: "n1", Name: "b"}}}},
		Edges:   []Edge{{From: "n0", To: "n1"}},
		Columns: []string{"name"},
		Rows:    [][]string{{"a"}},
		Notices: []string{"note"},
	}
	clone := original.Clone()
	clone.Roots[0].Name = "mutated"
	clone.Roots[0].Children[0].Name = "mutated"
	clone.Edges[0].To = "n9"
	clone.Columns[0] = "mutated"
	clone.Rows[0][0] = "mutated"
	clone.Notices[0] = "mutated"
	if original.Roots[0].Name != "a" || original.Roots[0].Children[0].Name != "b" {
		t.Error("roots shared with clone")
	}
	if original.Edges[0].To != "n1" || original.Columns[0] != "name" {
		t.Error("edges or columns shared with clone")
	}
	if original.Rows[0][0] != "a" || original.Notices[0] != "note" {
		t.Error("rows or notices shared with clone")
	}
	var nothing *Rendering
	if nothing.Clone() != nil {
		t.Error("nil rendering clones to a value")
	}
}
