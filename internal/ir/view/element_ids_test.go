package view

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestRenderedElementsCarryTheLinkIdentity pins the durable id of nodes, ports
// and edges to the identity a written form's {id} link template fills.
func TestRenderedElementsCarryTheLinkIdentity(t *testing.T) {
	renderer, index := loadFixture(t, "interconnection.sysml")
	rendering, err := renderer.Render(lookup(t, index, "PlantViews::loopView"))
	if err != nil {
		t.Fatal(err)
	}
	sites := renderer.Sites(func(o Origin) (string, source.Pos, bool) {
		return "model/" + o.Doc, source.Pos{Line: 1, Col: 1}, true
	})

	var nodes, ports, edges int
	check := func(kind string, origin Origin, id string) {
		t.Helper()
		if id == "" {
			return
		}
		site, ok := sites(origin)
		if !ok {
			t.Errorf("%s %q has element id %q but no source site", kind, origin.Doc, id)
			return
		}
		if site.ID != id {
			t.Errorf("%s element id = %q, want the {id} link value %q", kind, id, site.ID)
		}
		if filled := fillLinkTemplate("{id}", site); filled != id {
			t.Errorf("{id} fills to %q, want %q", filled, id)
		}
	}
	var walk func([]*Node)
	walk = func(list []*Node) {
		for _, node := range list {
			if node == nil {
				continue
			}
			nodes++
			check("node", node.Origin, node.ElementID)
			for _, port := range node.Ports {
				ports++
				check("port", port.Origin, port.ElementID)
			}
			for _, compartment := range node.Compartments {
				walk(compartment.Rows)
			}
			walk(node.Children)
		}
	}
	walk(rendering.Roots)
	for _, edge := range rendering.Edges {
		edges++
		check("edge", edge.Origin, edge.ElementID)
	}
	if nodes == 0 || ports == 0 || edges == 0 {
		t.Fatalf("fixture rendered %d nodes, %d ports and %d edges; want all three", nodes, ports, edges)
	}

	data := rendering.DataWith(Options{Ports: PortsFull})
	var dataNodes, dataPorts, dataEdges int
	for _, node := range data.Nodes {
		dataNodes++
		if node.ElementID == "" {
			continue
		}
		site, ok := sites(node.Origin)
		if !ok || site.ID != node.ElementID {
			t.Errorf("node data %q has element id %q, want %q", node.Name, node.ElementID, site.ID)
		}
		for _, port := range node.Ports {
			dataPorts++
			if port.ElementID != "" {
				if site, ok := sites(port.Origin); !ok || site.ID != port.ElementID {
					t.Errorf("port data %q has element id %q, want %q", port.Name, port.ElementID, site.ID)
				}
			}
		}
	}
	for _, edge := range data.Edges {
		dataEdges++
		if edge.ElementID == "" {
			continue
		}
		if site, ok := sites(edge.Origin); !ok || site.ID != edge.ElementID {
			t.Errorf("edge data %q has element id %q, want %q", edge.Kind, edge.ElementID, site.ID)
		}
	}
	if dataNodes != nodes || dataPorts != ports || dataEdges != edges {
		t.Errorf("data holds %d nodes, %d ports, %d edges; rendering holds %d, %d, %d",
			dataNodes, dataPorts, dataEdges, nodes, ports, edges)
	}

	written, err := rendering.WriteViewWith(FormD2, Options{Links: Links{Template: "https://example.com/{id}", Sites: sites}})
	if err != nil {
		t.Fatal(err)
	}
	var anchors int
	for _, node := range data.Nodes {
		if node.ElementID == "" {
			continue
		}
		site, ok := sites(node.Origin)
		if !ok {
			continue
		}
		if want := fillLinkTemplate("https://example.com/{id}", site); !strings.Contains(written, want) {
			t.Errorf("d2 form has no anchor to the element id %q of node %q", node.ElementID, node.Name)
		} else {
			anchors++
		}
	}
	if anchors == 0 {
		t.Error("no d2 anchor was written from an element id")
	}
}

// TestAnonymousConnectorsHaveDistinctElementIDs pins one durable id per unnamed
// connector that resolves to its declaration and written {id} anchor.
func TestAnonymousConnectorsHaveDistinctElementIDs(t *testing.T) {
	const doc = "anonymous-connector-ids.sysml"
	renderer, index := loadFixture(t, doc)
	rendering, err := renderer.Render(lookup(t, index, "AnonymousConnectors::wiring"))
	if err != nil {
		t.Fatal(err)
	}
	scope := index.DocumentRoot(doc)
	byName := make(map[string]*symbols.Symbol)
	for sym, name := range renderer.derivePositional(doc) {
		byName[name] = sym
	}

	seen := make(map[string]bool)
	var edges int
	for _, edge := range rendering.Edges {
		if edge.Kind != EdgeConnection {
			continue
		}
		edges++
		if edge.ElementID == "" {
			t.Fatalf("connection %q has no durable id", edge.Kind)
		}
		if seen[edge.ElementID] {
			t.Fatalf("two unnamed connections share the element id %q", edge.ElementID)
		}
		seen[edge.ElementID] = true

		sym := scope.DeclaredAt(edge.Origin.Span)
		if sym == nil {
			t.Fatalf("no declaration at %v", edge.Origin.Span)
		}
		positional := renderer.positionalOf(doc, sym)
		if positional == "" {
			t.Fatalf("connection %q has no positional identity", edge.Kind)
		}
		if want := identity.DerivedElementID(positional); edge.ElementID != want {
			t.Errorf("connection id = %q, want the positional identity %q", edge.ElementID, want)
		}
		if resolved := byName[positional]; resolved != sym {
			t.Errorf("positional identity %q resolves to %v, want the connection at %v", positional, resolved, edge.Origin.Span)
		}
	}
	if edges < 2 {
		t.Fatalf("fixture rendered %d connections; want at least two unnamed ones", edges)
	}

	sites := renderer.Sites(func(o Origin) (string, source.Pos, bool) {
		return "model/" + o.Doc, source.Pos{Line: 1, Col: 1}, true
	})
	for _, edge := range rendering.Edges {
		if edge.Kind != EdgeConnection {
			continue
		}
		site, ok := sites(edge.Origin)
		if !ok || site.ID != edge.ElementID {
			t.Errorf("written {id} anchor %q differs from structured id %q", site.ID, edge.ElementID)
		}
	}
}

// TestWalkNodesVisitsSharedNodesOnce pins linear dedup over folded rows and
// expanded type trees, which share node pointers.
func TestWalkNodesVisitsSharedNodesOnce(t *testing.T) {
	shared := &Node{ID: "shared"}
	root := &Node{ID: "root", Children: []*Node{shared, {ID: "wrapper", Children: []*Node{shared}}}}
	root.Compartments = []Compartment{{Name: "parts", Rows: []*Node{shared}}}
	var visited []string
	walkNodes([]*Node{root}, func(node *Node) { visited = append(visited, node.ID) })
	if !reflect.DeepEqual(visited, []string{"root", "shared", "wrapper"}) {
		t.Errorf("walkNodes visited %v, want root, shared and wrapper once each", visited)
	}
}

// TestPositionalDerivationDoesNotChangeRendering pins that deriving reported
// identities does not change what later renderings draw.
func TestPositionalDerivationDoesNotChangeRendering(t *testing.T) {
	const file = "spec-compartments.sysml"
	renderer, index := loadFixture(t, file)
	view := lookup(t, index, "HeadingViews::headings")
	first, err := renderer.DrawnIn(view)
	if err != nil {
		t.Fatal(err)
	}
	second, err := renderer.DrawnIn(view)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("first drawn query differs from the second: %#v vs %#v", first, second)
	}
}

func TestRenderedElementIDsAreStableAcrossRenderings(t *testing.T) {
	idsOf := func() []string {
		renderer, index := loadFixture(t, "interconnection.sysml")
		rendering, err := renderer.Render(lookup(t, index, "PlantViews::loopView"))
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, node := range rendering.Roots {
			ids = append(ids, node.ElementID)
			for _, port := range node.Ports {
				ids = append(ids, port.ElementID)
			}
		}
		for _, edge := range rendering.Edges {
			ids = append(ids, edge.ElementID)
		}
		return ids
	}
	first, second := idsOf(), idsOf()
	if len(first) == 0 {
		t.Fatal("no identity-bearing elements in the fixture")
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("element id %d = %q then %q; identity is not stable across renderings", i, first[i], second[i])
		}
	}
}

func TestResolveElementIDsToleratesNilAndUnlocatedOrigins(t *testing.T) {
	var nilRenderer *Renderer
	rendering := &Rendering{Roots: []*Node{{ID: "n", Ports: []Port{{ID: "p"}}}}, Edges: []Edge{{From: "n", To: "n"}}}
	nilRenderer.resolveElementIDs(rendering)
	(&Renderer{}).resolveElementIDs(rendering)
	for _, node := range rendering.Roots {
		if node.ElementID != "" {
			t.Errorf("unlocated node has element id %q", node.ElementID)
		}
		for _, port := range node.Ports {
			if port.ElementID != "" {
				t.Errorf("unlocated port has element id %q", port.ElementID)
			}
		}
	}
	if rendering.Edges[0].ElementID != "" {
		t.Errorf("unlocated edge has element id %q", rendering.Edges[0].ElementID)
	}
}
