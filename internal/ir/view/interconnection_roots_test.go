package view

import (
	"strings"
	"testing"
)

// everyNode collects the nodes of a rendering at every depth.
func everyNode(roots []*Node) []*Node {
	var out []*Node
	var walk func(*Node)
	walk = func(node *Node) {
		out = append(out, node)
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	return out
}

// checkSingleDraw asserts each node is drawn once and the edges join the
// nested port nodes, across the node tree and every written form.
func checkSingleDraw(t *testing.T, rendering *Rendering) {
	t.Helper()
	if len(rendering.Roots) != 1 {
		t.Fatalf("roots = %d, want 1: %v", len(rendering.Roots), nodeNames(rendering.Roots))
	}
	counts := map[string]int{}
	byName := map[string]*Node{}
	for _, node := range everyNode(rendering.Roots) {
		counts[node.Name]++
		byName[node.Name] = node
		if strings.Contains(node.Detail, "already shown") {
			t.Errorf("node %s detail = %q, want no stub", node.Name, node.Detail)
		}
	}
	for _, name := range []string{"pa", "pb"} {
		if counts[name] != 1 {
			t.Errorf("node %q drawn %d times, want once; counts: %v", name, counts[name], counts)
		}
	}
	if len(rendering.Edges) != 1 {
		t.Fatalf("edges = %v, want the one connection", rendering.Edges)
	}
	edge := rendering.Edges[0]
	pa, pb := byName["pa"], byName["pb"]
	if pa == nil || pb == nil || (edge.From != pa.ID || edge.To != pb.ID) && (edge.From != pb.ID || edge.To != pa.ID) {
		t.Errorf("edge %s -> %s does not join the nested port nodes: %+v", edge.From, edge.To, rendering.Roots)
	}
}

// checkPortDrawnOnce asserts each form draws one port node under its declared
// name, and no exposed port a second time under its qualified name.
func checkPortDrawnOnce(t *testing.T, label, form, portLiteral string) {
	t.Helper()
	if got := strings.Count(form, portLiteral); got != 1 {
		t.Errorf("%s draws %q %d times, want once:\n%s", label, portLiteral, got, form)
	}
	for _, bad := range []string{"a::pa", "b::pb", "already shown"} {
		if strings.Contains(form, bad) {
			t.Errorf("%s contains %q:\n%s", label, bad, form)
		}
	}
}

// An exposed feature an exposed container already draws nested in it is no
// second root: it is drawn once, nested, and the connections join that node.
func TestExposedNestedFeaturesAreNoSecondRoots(t *testing.T) {
	rendering := render(t, "interconnection-exposed.sysml", "FixtureViews::dupIBD")
	checkSingleDraw(t, rendering)
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkPortDrawnOnce(t, "DOT", dot, `xlabel="pa : P"`)
	checkPortDrawnOnce(t, "DOT", dot, `xlabel="pb : P"`)
	cameo, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOTWith: %v", err)
	}
	checkPortDrawnOnce(t, "cameo DOT", cameo, `xlabel="pa : P"`)
	checkPortDrawnOnce(t, "cameo DOT", cameo, `xlabel="pb : P"`)
	mermaid := rendering.Mermaid()
	checkPortDrawnOnce(t, "Mermaid", mermaid, "pa : P")
	checkPortDrawnOnce(t, "Mermaid", mermaid, "pb : P")
	plantuml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	checkPortDrawnOnce(t, "PlantUML", plantuml, "**pa : P**")
	checkPortDrawnOnce(t, "PlantUML", plantuml, "**pb : P**")
}

// The one node a nested feature is drawn as keeps the geometry its Layout
// states, positioned where a second root would have carried it.
func TestExposedNestedFeaturesKeepTheirLayout(t *testing.T) {
	rendering := render(t, "interconnection-exposed.sysml", "FixtureViews::dupIBD")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{`xlabel="pa : P", pos="170,-90!"`, `xlabel="pb : P", pos="230,-90!"`} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
}

// An exposed feature stays a root when no exposed element draws it nested.
func TestNestedFeatureWithoutDrawnParentStaysARoot(t *testing.T) {
	rendering := render(t, "interconnection-exposed.sysml", "FixtureViews::portsOnly")
	if len(rendering.Roots) != 2 {
		t.Fatalf("roots = %d, want 2: %v", len(rendering.Roots), nodeNames(rendering.Roots))
	}
	names := map[string]*Node{}
	for _, root := range rendering.Roots {
		names[root.Name] = root
	}
	for _, want := range []string{"pa", "pb"} {
		var found bool
		for name := range names {
			if strings.HasSuffix(name, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no root named for %q; roots: %v", want, nodeNames(rendering.Roots))
		}
	}
	if len(rendering.Edges) != 1 {
		t.Fatalf("edges = %v, want the one connection", rendering.Edges)
	}
	edge := rendering.Edges[0]
	var pa, pb *Node
	for _, root := range rendering.Roots {
		if strings.HasSuffix(root.Name, "pa") {
			pa = root
		}
		if strings.HasSuffix(root.Name, "pb") {
			pb = root
		}
	}
	if (edge.From != pa.ID || edge.To != pb.ID) && (edge.From != pb.ID || edge.To != pa.ID) {
		t.Errorf("edge %s -> %s does not join the exposed port roots %s, %s", edge.From, edge.To, pa.ID, pb.ID)
	}
}

// An exposed element an interconnection rendering does not draw — a package is
// a notice, a connector an edge — does not suppress what is nested in it: a
// part exposed beside the package holding it still stands as the root.
func TestExposedPackageDoesNotSuppressTheFeatureItHolds(t *testing.T) {
	rendering := render(t, "interconnection-exposed.sysml", "FixtureViews::packageIBD")
	if len(rendering.Roots) != 1 {
		t.Fatalf("roots = %d, want 1: %v", len(rendering.Roots), nodeNames(rendering.Roots))
	}
	root := rendering.Roots[0]
	if root.Kind != "part def" || !strings.HasSuffix(root.Name, "Whole") {
		t.Errorf("root = %s %s, want the part def Whole", root.Kind, root.Name)
	}
	names := nodeNames(rendering.Roots)
	for _, want := range []string{"a", "b", "pa", "pb"} {
		if !names[want] {
			t.Errorf("no nested node %q; nodes: %v", want, names)
		}
	}
	if len(rendering.Edges) != 1 {
		t.Fatalf("edges = %v, want the one connection", rendering.Edges)
	}
	var pa, pb *Node
	for _, node := range everyNode(rendering.Roots) {
		switch node.Name {
		case "pa":
			pa = node
		case "pb":
			pb = node
		}
	}
	edge := rendering.Edges[0]
	if (edge.From != pa.ID || edge.To != pb.ID) && (edge.From != pb.ID || edge.To != pa.ID) {
		t.Errorf("edge %s -> %s does not join the nested port nodes %s, %s", edge.From, edge.To, pa.ID, pb.ID)
	}
	var noticed bool
	for _, notice := range rendering.Notices {
		if strings.Contains(notice, "has no place in an interconnection rendering") && strings.Contains(notice, "Fixture") {
			noticed = true
		}
	}
	if !noticed {
		t.Errorf("no notice for the exposed package; notices: %v", rendering.Notices)
	}
}

// The order elements are exposed in does not change what is drawn nested:
// ports exposed before the part that contains them are still drawn once.
func TestExposeOrderDoesNotDuplicateNestedFeatures(t *testing.T) {
	rendering := render(t, "interconnection-exposed.sysml", "FixtureViews::orderedIBD")
	checkSingleDraw(t, rendering)
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkPortDrawnOnce(t, "DOT", dot, `xlabel="pa : P"`)
	checkPortDrawnOnce(t, "DOT", dot, `xlabel="pb : P"`)
}
