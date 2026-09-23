package view

import (
	"maps"
	"slices"
	"testing"
)

// allNodes walks a rendering's nodes in depth-first order.
func allNodes(roots []*Node) []*Node {
	var out []*Node
	for _, node := range roots {
		out = append(out, node)
		out = append(out, allNodes(node.Children)...)
	}
	return out
}

// A tree shows the model, not the picture of it: a view's `render` member and
// every DiagramLayout annotation are left out, whether stated inline on the
// element, about a member, or in a view's body. Ordinary metadata stays.
func TestTreeLeavesOutLayoutAndRenderMembers(t *testing.T) {
	rendering := render(t, "bookkeeping.sysml", "BudgetViews::budgetView")
	names := nodeNames(rendering.Roots)
	for _, want := range []string{"Mirror", "errorReq", "errorCBE", "Approved", "details", "asTree"} {
		if !names[want] {
			t.Errorf("node %q missing; nodes: %v", want, slices.Sorted(maps.Keys(names)))
		}
	}
	for _, node := range allNodes(rendering.Roots) {
		switch {
		case node.Kind == "metadata" && node.Type != "Approved":
			t.Errorf("layout annotation drawn as node %q : %q", node.Name, node.Type)
		case node.Kind == "render" || node.Name == "asTreeDiagram":
			t.Errorf("render member drawn as node %q", node.Name)
		}
	}
	for _, name := range []string{"x", "y", "width", "height", "unit", "placed"} {
		if names[name] {
			t.Errorf("layout bookkeeping %q drawn as a node", name)
		}
	}
	details := findNode(t, rendering.Roots, "details")
	if len(details.Children) != 0 {
		t.Errorf("view details has children %v, want none", nodeNames(details.Children))
	}
}

// What a tree does not draw, no Layout can position.
func TestLayoutAnnotationsAreNotDrawn(t *testing.T) {
	r, idx := loadFixtures(t, "bookkeeping.sysml")
	for _, fqn := range []string{"Budget::Mirror::placed", "Budget::Mirror::details::asTreeDiagram"} {
		syms := idx.LookupQualified(fqn)
		if len(syms) == 0 {
			continue
		}
		if node, _ := r.draws(KindTree, syms[0]); node {
			t.Errorf("%s is drawn by a tree rendering", fqn)
		}
	}
	if node, _ := r.draws(KindTree, lookup(t, idx, "Budget::asTree")); !node {
		t.Error("Budget::asTree is a rendering usage outside a view; a tree draws it")
	}
}
