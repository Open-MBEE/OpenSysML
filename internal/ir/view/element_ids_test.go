package view

import (
	"strings"
	"testing"

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
