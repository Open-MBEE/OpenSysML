package view

import (
	"maps"
	"slices"
	"strings"
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

// A name the migration made up (MigrationMetadata::SynthesizedName) keys its
// node but is not one a picture shows: the node carries the bit, as does the
// language's own start of an action's flow, and an edge with no text but such
// a name carries no label, in every rendering kind.
func TestSynthesizedNamesAreReadFromTheModel(t *testing.T) {
	synthesized := map[string]bool{"start": true, "'fork'": true, "final": true, "wheel": true, "'start to call'": true, "'call to fork'": true,
		"'fork to log'": true, "'fork to final'": true, "'Idle accept Go then Running'": true, "'engine to wheel'": true}
	authored := []string{"call", "log", "Idle", "Running", "engine", "Track", "Modes", "Vehicle"}
	for _, view := range []string{"MigratedViews::trackView", "MigratedViews::modesView", "MigratedViews::vehicleView", "MigratedViews::treeView"} {
		rendering := render(t, "synthesized.sysml", view)
		for _, node := range allNodes(rendering.Roots) {
			if node.NameSynthesized != synthesized[node.Name] {
				t.Errorf("%s: node %q NameSynthesized = %v, want %v", view, node.Name, node.NameSynthesized, synthesized[node.Name])
			}
		}
		names := nodeNames(rendering.Roots)
		for _, name := range authored {
			if !names[name] && view != "MigratedViews::treeView" {
				continue
			}
			if !names[name] {
				t.Errorf("%s: node %q missing; nodes: %v", view, name, slices.Sorted(maps.Keys(names)))
			}
		}
	}

	track := render(t, "synthesized.sysml", "MigratedViews::trackView")
	labels := edgeLabels(track)
	for _, label := range []string{"'start to call'", "'call to fork'", "'fork to log'", "'fork to final'", "start to call", "fork to log"} {
		if labels[label] {
			t.Errorf("succession's synthesized name %q labels an edge", label)
		}
	}
	if !labels["finish"] {
		t.Errorf("authored succession name finish labels no edge; labels: %v", slices.Sorted(maps.Keys(labels)))
	}

	modes := render(t, "synthesized.sysml", "MigratedViews::modesView")
	labels = edgeLabels(modes)
	for _, want := range []string{"accept Go", "accept Stop"} {
		if !labels[want] {
			t.Errorf("transition trigger %q labels no edge; labels: %v", want, slices.Sorted(maps.Keys(labels)))
		}
	}
	if labels["'Idle accept Go then Running'"] {
		t.Error("transition's synthesized name labels an edge over its trigger")
	}

	vehicle := render(t, "synthesized.sysml", "MigratedViews::vehicleView")
	labels = edgeLabels(vehicle)
	if labels["'engine to wheel'"] || labels["engine to wheel"] {
		t.Errorf("connection's synthesized name labels an edge; labels: %v", slices.Sorted(maps.Keys(labels)))
	}
	if !labels["drive"] || !labels["connection"] {
		t.Errorf("connection labels = %v, want the authored name drive and the keyword for the synthesized one", slices.Sorted(maps.Keys(labels)))
	}
	dot, err := vehicle.DOT()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dot, "label=<<b>: Wheel</b>") || strings.Contains(dot, ">wheel") {
		t.Errorf("part with a synthesized name is not drawn as `: Wheel` alone:\n%s", dot)
	}
}
