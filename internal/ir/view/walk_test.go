package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"strings"
	"testing"
)

// A view's depth is its own depth attribute, else the nearest view definition's;
// 0 is no bound, and the caller's depth overrides them all for that render.
func TestWalkDepthNearestWinsAndZeroIsUnbounded(t *testing.T) {
	r, idx := loadFixtures(t, "depth-tree.sysml")
	cases := []struct {
		view     string
		override int
		deepest  string
	}{
		{"DepthViews::depth1", 0, "wheel"},
		{"DepthViews::depth2", 0, "hub"},
		{"DepthViews::depth3", 0, "bearing"},
		{"DepthViews::unlimited", 0, "grade"},
		{"DepthViews::unlimited", 1, "wheel"},
		{"DepthViews::depth1", 3, "bearing"},
	}
	for _, tc := range cases {
		r.SetDepth(tc.override)
		rendering, err := r.Render(lookup(t, idx, tc.view))
		if err != nil {
			t.Fatalf("render %s: %v", tc.view, err)
		}
		names := nodeNames(rendering.Roots)
		if !names[tc.deepest] {
			t.Errorf("%s with override %d: %s is not drawn; nodes: %v", tc.view, tc.override, tc.deepest, names)
		}
		if tc.deepest != "grade" && names["grade"] {
			t.Errorf("%s with override %d draws grade, below the bound", tc.view, tc.override)
		}
	}
}

// A depth bound is noticed once, counting the elements it kept from the drawing.
func TestWalkDepthIsNoticedOnce(t *testing.T) {
	rendering := render(t, "depth-tree.sysml", "DepthViews::depth1")
	var cuts int
	for _, notice := range rendering.Notices {
		if strings.Contains(notice, "nested deeper than 1 level") {
			cuts++
			if !strings.HasPrefix(notice, "3 elements") {
				t.Errorf("notice counts %q, want the 3 elements below wheel and engine, fuelIn kept", notice)
			}
		}
	}
	if cuts != 1 {
		t.Errorf("depth noticed %d times, want once: %v", cuts, rendering.Notices)
	}
}

// The view's filter conditions prune the walk: a member a condition rejects is
// not drawn, nor anything under it, while the exposed root is admitted as before.
func TestWalkFiltersPruneContainedMembers(t *testing.T) {
	rendering := render(t, "walk-filter.sysml", "FilteredWalk::structure")
	names := nodeNames(rendering.Roots)
	for _, want := range []string{"FilteredWalk::rover", "chassis", "axle", "radio", "gps"} {
		if !names[want] {
			t.Errorf("%s is not drawn; nodes: %v", want, names)
		}
	}
	for _, hidden := range []string{"mass", "length", "telemetry", "tx", "fix"} {
		if names[hidden] {
			t.Errorf("%s is drawn, though the view's filters reject it; nodes: %v", hidden, names)
		}
	}
}

// A connector ending at a pruned element is reported, not drawn, and not
// rerouted to the pruned element's owner.
func TestWalkFilteredConnectorEndIsNoticed(t *testing.T) {
	rendering := render(t, "walk-filter.sysml", "FilteredWalk::links")
	if len(rendering.Edges) != 0 {
		t.Errorf("edges = %+v, want none: telemetry is pruned", rendering.Edges)
	}
	if !strings.Contains(strings.Join(rendering.Notices, "\n"), "which the view's filter conditions exclude") {
		t.Errorf("notices = %v, want the pruned connector end reported", rendering.Notices)
	}
}

// A connector crossing the depth bound ends at the drawn element standing in
// for what lies below it, and the ports of an element at the bound stay pinned.
func TestWalkDepthKeepsPortsAndReroutesCrossingConnectors(t *testing.T) {
	rendering := render(t, "depth-interconnection.sysml", "DepthLinks::links")
	names := nodeNames(rendering.Roots)
	for _, want := range []string{"plant", "status", "controller", "cmd"} {
		if !names[want] {
			t.Errorf("%s is not drawn; nodes: %v", want, names)
		}
	}
	for _, hidden := range []string{"pump", "tank", "out"} {
		if names[hidden] {
			t.Errorf("%s is drawn below the bound; nodes: %v", hidden, names)
		}
	}
	if len(rendering.Edges) != 2 {
		t.Errorf("edges = %d, want the two connectors from controller", len(rendering.Edges))
	}
}

// A collapsed Layout folds an element's children away in every form, keeping
// the collapsed tag; its ports stay, and a connector still ends at them.
func TestCollapsedLayoutFoldsChildrenInEveryForm(t *testing.T) {
	for _, viewName := range []string{"FoldedViews::folded", "FoldedViews::links"} {
		rendering := render(t, "layout-collapsed.sysml", viewName)
		for _, form := range []Form{FormText, FormMermaid, FormDot, FormPlantUML, FormD2} {
			artifact, err := rendering.Write(form)
			if err != nil {
				t.Fatalf("%s as %s: %v", viewName, form, err)
			}
			for _, folded := range []string{"piston", "pump"} {
				if strings.Contains(artifact, folded) {
					t.Errorf("%s as %s draws %s under the collapsed engine:\n%s", viewName, form, folded, artifact)
				}
			}
			if !strings.Contains(artifact, "collapsed") {
				t.Errorf("%s as %s lost the collapsed tag:\n%s", viewName, form, artifact)
			}
		}
		if rendering.Kind == KindInterconnection && len(rendering.Edges) != 1 {
			t.Errorf("%s draws %d edges, want the connector to engine's port", viewName, len(rendering.Edges))
		}
	}
}

// The text form omits the members a Layout's showParts = false hides, as DOT does.
func TestTextOmitsHiddenCompartments(t *testing.T) {
	rendering := render(t, "layout-collapsed.sysml", "FoldedViews::noParts")
	text, err := rendering.Write(FormText)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{"wheel", "engine", "tank"} {
		if strings.Contains(text, hidden) {
			t.Errorf("text lists %s, which showParts = false hides:\n%s", hidden, text)
		}
	}
}

// A type filter prunes the ports a part has from its definition as well as
// the ones it declares: none is pinned, and a connector ending at one is noticed.
func TestWalkFiltersPruneInheritedPorts(t *testing.T) {
	rendering := render(t, "walk-filter.sysml", "FilteredWalk::partsOnly")
	var pins int
	var walk func(nodes []*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			pins += len(node.Ports)
			walk(node.Children)
		}
	}
	walk(rendering.Roots)
	if pins != 0 || len(rendering.Edges) != 0 {
		t.Errorf("%d pins and %d edges drawn, want none: the view admits part usages only", pins, len(rendering.Edges))
	}
}

// A chain end naming a pruned member of a drawn element ends no edge at that
// element; it is noticed as excluded.
func TestWalkFilteredChainEndIsNotReroutedToItsOwner(t *testing.T) {
	rendering := render(t, "walk-filter.sysml", "FilteredWalk::links")
	if len(rendering.Edges) != 0 {
		t.Errorf("edges = %+v, want none: tx and fix are pruned", rendering.Edges)
	}
	var excluded int
	for _, notice := range rendering.Notices {
		if strings.Contains(notice, "which the view's filter conditions exclude") {
			excluded++
		}
	}
	if excluded != 2 {
		t.Errorf("%d connectors noticed as excluded, want both: %v", excluded, rendering.Notices)
	}
}

// A tree keeps the ports of an element at the depth bound, as the
// interconnection does: they count at their owner's depth.
func TestWalkTreeKeepsPortsAtTheBound(t *testing.T) {
	rendering := render(t, "depth-tree.sysml", "DepthViews::depth1")
	names := nodeNames(rendering.Roots)
	if !names["fuelIn"] || names["piston"] {
		t.Errorf("at depth 1 the tree draws fuelIn=%v piston=%v; want the port kept and the part cut", names["fuelIn"], names["piston"])
	}
}

// An exposed element beneath a collapsed exposed element is drawn as a root of
// its own, since the fold draws nothing beneath the outer one.
func TestWalkCollapsedRootKeepsExposedChildAsRoot(t *testing.T) {
	rendering := render(t, "layout-collapsed.sysml", "FoldedViews::both")
	var roots []string
	for _, root := range rendering.Roots {
		roots = append(roots, root.Name)
	}
	if len(rendering.Roots) != 2 || !nodeNames(rendering.Roots)["piston"] {
		t.Errorf("roots = %v (piston drawn: %v); want vehicle and engine, engine with its members", roots, nodeNames(rendering.Roots)["piston"])
	}
}

// A mixed diagram's structural subtrees carry their depth cuts into the notices.
func TestWalkMixedCarriesDepthCuts(t *testing.T) {
	r, idx := loadFixtures(t, "depth-tree.sysml")
	r.SetDepth(1)
	rendering, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, "DepthViews::vehicle")}, KindMixed, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(rendering.Notices, "\n"), "nested deeper than 1 level not shown") {
		t.Errorf("notices = %v, want the depth cut noticed", rendering.Notices)
	}
}

// A Layout binding only collapsed folds the element without placing it: no
// form switches to positioned mode, so the siblings stay drawn and nothing is
// left undrawn for want of a position.
func TestFoldOnlyLayoutLeavesTheFormToPlace(t *testing.T) {
	for _, viewName := range []string{"FoldedViews::foldOnly", "FoldedViews::foldOnlyLinks"} {
		rendering := render(t, "layout-collapsed.sysml", viewName)
		names := nodeNames(rendering.Roots)
		for _, drawn := range []string{"wheel", "hub", "engine", "tank"} {
			if !names[drawn] {
				t.Errorf("%s does not draw %s; nodes: %v", viewName, drawn, names)
			}
		}
		for _, form := range []Form{FormText, FormMermaid, FormDot, FormPlantUML, FormD2} {
			artifact, err := rendering.Write(form)
			if err != nil {
				t.Fatalf("%s as %s: %v", viewName, form, err)
			}
			for _, folded := range []string{"piston", "pump"} {
				if strings.Contains(artifact, folded) {
					t.Errorf("%s as %s draws %s under the collapsed engine:\n%s", viewName, form, folded, artifact)
				}
			}
			for _, kept := range []string{"wheel", "tank", "collapsed"} {
				if !strings.Contains(artifact, kept) {
					t.Errorf("%s as %s lost %s:\n%s", viewName, form, kept, artifact)
				}
			}
			if strings.Contains(artifact, "without a position") || strings.Contains(artifact, " at (") {
				t.Errorf("%s as %s treats the fold-only Layout as a place:\n%s", viewName, form, artifact)
			}
		}
	}
}

// A whitelist selects exactly the marked elements: `expose rover::**[@Shown]`
// draws each as a root with none of the ancestors between them, and
// `filter @Shown` on an unmarked root leaves the view empty rather than
// adding them, since a filter condition only ever prunes.
func TestWalkWhitelistDrawsTheMarkedElementsAlone(t *testing.T) {
	picked := render(t, "walk-filter.sysml", "FilteredWalk::picked")
	roots := map[string]bool{}
	for _, root := range picked.Roots {
		roots[root.Name] = true
	}
	if len(roots) != 2 || !roots["FilteredWalk::rover::chassis::axle"] || !roots["FilteredWalk::rover::radio"] {
		t.Errorf("picked roots = %v, want the marked axle and radio", roots)
	}
	names := nodeNames(picked.Roots)
	for _, unmarked := range []string{"FilteredWalk::rover", "chassis", "rx"} {
		if names[unmarked] {
			t.Errorf("picked draws %s, which carries no mark; nodes: %v", unmarked, names)
		}
	}

	rejected := render(t, "walk-filter.sysml", "FilteredWalk::wholeRejected")
	if len(rejected.Roots) != 0 {
		t.Errorf("wholeRejected draws %v, want nothing: the root rover carries no mark", nodeNames(rejected.Roots))
	}
	if reason := rejected.EmptyReason(); !strings.Contains(reason, "rejected by its filter @Shown") {
		t.Errorf("wholeRejected empty reason = %q, want one naming the filter", reason)
	}
}
