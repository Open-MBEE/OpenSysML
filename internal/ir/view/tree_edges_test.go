package view

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// treeEdgeLines spells a tree's edges by the names of the nodes they join,
// their kind and their label, in the order drawn.
func treeEdgeLines(rendering *Rendering) []string {
	names := map[string]string{}
	var visit func(nodes []*Node)
	visit = func(nodes []*Node) {
		for _, node := range nodes {
			names[node.ID] = node.Name
			visit(node.Children)
		}
	}
	visit(rendering.Roots)
	var lines []string
	for _, edge := range rendering.Edges {
		line := fmt.Sprintf("%s %s %s", names[edge.From], edge.Kind, names[edge.To])
		if edge.Label != "" {
			line += ": " + edge.Label
		}
		lines = append(lines, line)
	}
	return lines
}

// A tree draws the relationships between the elements it has nodes for: a
// composition from an element to the definition typing each part it owns,
// labelled with the part's name and multiplicity, a reference for a `ref` or an
// attribute, a specialization, redefinition or subsetting to its general, and
// the typing of a usage whose owner is not drawn. A typing a drawn owner's
// composition stands for is not drawn twice, nor is a composition to a
// definition nested in its owner, nor an edge to an element the tree does not
// draw or a type that does not resolve.
func TestTreeDrawsTheRelationshipsBetweenItsNodes(t *testing.T) {
	rendering := render(t, "tree-edges.sysml", "FleetViews::structure")
	assertRenderingEdgeEndpoints(t, rendering)
	want := []string{
		"Fleet::Vehicle composition Fleet::Chassis: chassis",
		"Fleet::Vehicle composition Fleet::Wheel: wheels[4]",
		"Fleet::Vehicle reference Fleet::Driver: driver[0..1]",
		"Fleet::Vehicle reference Fleet::Mass: total",
		"Fleet::SportsCar specialization Fleet::Vehicle",
		"Fleet::SportsCar composition Fleet::LightChassis: chassis",
		"Fleet::SportsCar composition Fleet::Wheel: spare",
		"chassis specialization chassis: redefines",
		"spare specialization wheels: subsets",
		"Fleet::Chassis reference Fleet::Mass: mass",
		"Fleet::LightChassis specialization Fleet::Chassis",
		"mass specialization mass: redefines",
		"Fleet::fleetCar typing Fleet::Vehicle",
	}
	if got := treeEdgeLines(rendering); !reflect.DeepEqual(got, want) {
		t.Errorf("edges =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none: a type that does not resolve is left undrawn silently", rendering.Notices)
	}
}

// A composition edge stands for the usage it is labelled with, so the Route
// about that usage steers it and a layout check sees the usage drawn as an edge;
// a specialization has no member of its own and is left to the client to route.
func TestTreeCompositionEdgeIsRoutedByTheUsagesRoute(t *testing.T) {
	r, idx := loadFixture(t, "tree-edges.sysml")
	view := lookup(t, idx, "FleetViews::routed")
	rendering, err := r.Render(view)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var routed []string
	for _, edge := range rendering.Edges {
		if len(edge.Route) > 0 {
			routed = append(routed, edge.Label)
			if want := []Point{{100, 200}, {300, 200}}; !reflect.DeepEqual(edge.Route, want) {
				t.Errorf("route of %s = %v, want %v", edge.Label, edge.Route, want)
			}
		}
	}
	if !reflect.DeepEqual(routed, []string{"wheels[4]"}) {
		t.Errorf("routed edges = %v, want the wheels composition alone", routed)
	}
	drawn, err := r.DrawnIn(view)
	if err != nil {
		t.Fatalf("DrawnIn: %v", err)
	}
	if wheels := lookup(t, idx, "Fleet::Vehicle::wheels"); !drawn.Edge(wheels) || !drawn.Node(wheels) {
		t.Errorf("wheels drawn as edge %v, node %v; want both", drawn.Edge(wheels), drawn.Node(wheels))
	}
	if vehicle := lookup(t, idx, "Fleet::Vehicle"); drawn.Edge(vehicle) || !drawn.Node(vehicle) {
		t.Error("Vehicle is drawn as an edge; a definition's relationships are no member a Route can name")
	}
}

// A composition edge originates at the usage it stands for; a specialization,
// redefinition or typing edge at the name its clause relates the element to,
// which no symbol declares, so a client navigates to the clause and offers no
// route handles.
func TestTreeEdgeOriginsLocateTheRelationship(t *testing.T) {
	rendering := render(t, "tree-edges.sysml", "FleetViews::structure")
	sf := fixtureText(t, "tree-edges.sysml")
	lines := treeEdgeLines(rendering)
	got := map[string]string{}
	for i, edge := range rendering.Edges {
		if edge.Origin.Doc != "tree-edges.sysml" {
			t.Errorf("edge %s: origin document = %q", lines[i], edge.Origin.Doc)
			continue
		}
		got[lines[i]] = sf.Text(edge.Origin.Span)
	}
	for line, want := range map[string]string{
		"Fleet::Vehicle composition Fleet::Wheel: wheels[4]":   "part wheels[4] : Wheel;",
		"Fleet::Vehicle reference Fleet::Driver: driver[0..1]": "ref part driver[0..1] : Driver;",
		"Fleet::SportsCar specialization Fleet::Vehicle":       "Vehicle",
		"chassis specialization chassis: redefines":            "chassis",
		"spare specialization wheels: subsets":                 "wheels",
		"Fleet::LightChassis specialization Fleet::Chassis":    "Chassis",
		"Fleet::fleetCar typing Fleet::Vehicle":                "Vehicle",
	} {
		if text, ok := got[line]; !ok {
			t.Errorf("no edge %q among %v", line, lines)
		} else if text != want {
			t.Errorf("edge %q originates at %q, want %q", line, text, want)
		}
	}
}

// The edges of a tree reach into the subtrees its nested views draw, each
// node's Layout and each edge's Route read in the view that shows it.
func TestTreeEdgesReachNestedViews(t *testing.T) {
	rendering := render(t, "tree.sysml", "VehicleViews::vehicleView")
	if got, want := treeEdgeLines(rendering), []string{"Vehicles::Vehicle composition Vehicles::Engine: engine"}; !reflect.DeepEqual(got, want) {
		t.Errorf("edges = %v, want %v", got, want)
	}
}

// The text form lists a tree's edges as the relationships they are, in the
// class-diagram arrows the general graphs use.
func TestTreeTextListsRelationships(t *testing.T) {
	text := render(t, "tree-edges.sysml", "FleetViews::structure").Text()
	for _, want := range []string{
		"\nrelationships:\n",
		"  Fleet::Vehicle *-- Fleet::Wheel: wheels[4]\n",
		"  Fleet::Vehicle o-- Fleet::Driver: driver[0..1]\n",
		"  Fleet::SportsCar --|> Fleet::Vehicle\n",
		"  Fleet::fleetCar ..|> Fleet::Vehicle\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
}
