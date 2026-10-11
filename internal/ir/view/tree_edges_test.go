package view

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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

func TestTreeOmitsRelationshipsIntoNestedNodes(t *testing.T) {
	rendering := renderSource(t, "Views::structure", `package Model {
	part def General;
	part def Outer :> Outer::Inner, General {
		part def Inner;
	}
}
package Views {
	view structure {
		expose Model::Outer;
		expose Model::General;
	}
}`)
	if got, want := treeEdgeLines(rendering), []string{"Model::Outer specialization Model::General"}; !reflect.DeepEqual(got, want) {
		t.Errorf("edges = %q, want %q", got, want)
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

// A usage's Style colours the composition edge standing for it as it does
// the usage's node; its Note stays anchored to the node alone, not repeated
// on the edge.
func TestTreeCompositionEdgeWearsTheUsagesStyle(t *testing.T) {
	rendering := renderSource(t, "Views::styled", `package Model {
	part def Wheel;
	part def Car { part wheels[4] : Wheel; }
}
package Views {
	private import DiagramLayout::*;
	view styled {
		expose Model::Car;
		expose Model::Wheel;
		metadata Style about Model::Car::wheels { line = "#FF0000"; }
		metadata Note about Model::Car::wheels { text = "four of them"; x = 10; y = 20; }
	}
}`)
	if len(rendering.Edges) != 1 || rendering.Edges[0].Style == nil || rendering.Edges[0].Style.Line != "#FF0000" {
		t.Fatalf("edges = %+v, want one composition styled with the wheels' line", rendering.Edges)
	}
	wheels := rendering.Roots[0].Children[0]
	if wheels.Style == nil || wheels.Style.Line != "#FF0000" {
		t.Errorf("wheels node style = %+v, want the same line", wheels.Style)
	}
	if len(rendering.Notes) != 1 || rendering.Notes[0].Anchor != wheels.ID || rendering.Notes[0].EdgeFrom != "" {
		t.Errorf("notes = %+v, want the one note anchored to the wheels node", rendering.Notes)
	}
}

// A multiplicity bound that names a feature is spelled by that name when no
// source text is at hand to copy, as a literal one is by its value.
func TestTreeMultiplicityBoundsAreSpelledWithoutSourceText(t *testing.T) {
	r, idx := loadSources(t, []string{"inline.sysml"}, [][]byte{[]byte(`package Model {
	part def Wheel;
	part def Seat;
	part def Car {
		attribute axles : ScalarValues::Integer;
		attribute rows : ScalarValues::Integer;
		part wheels[axles] : Wheel;
		part seats[0..rows] : Seat;
	}
}
package Views {
	view parts { expose Model::*; }
}`)})
	r.text = nil
	rendering, err := r.Render(lookup(t, idx, "Views::parts"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var labels []string
	for _, edge := range rendering.Edges {
		labels = append(labels, edge.Label)
	}
	if want := []string{"wheels[axles]", "seats[0..rows]"}; !reflect.DeepEqual(labels, want) {
		t.Errorf("labels = %q, want %q", labels, want)
	}
}

// Two anonymous usages of one type are two memberships, each drawn.
func TestTreeDrawsEachAnonymousUsage(t *testing.T) {
	rendering := renderSource(t, "Views::pair", `package Model {
	part def B;
	part def A { part : B; part : B; }
}
package Views {
	view pair { expose Model::A; expose Model::B; }
}`)
	if got := treeEdgeLines(rendering); !reflect.DeepEqual(got, []string{"Model::A composition Model::B", "Model::A composition Model::B"}) {
		t.Errorf("edges = %q, want both anonymous compositions", got)
	}
}

// A member the depth bound leaves undrawn is drawn as no edge either: the
// tree says the member is not shown, and shows nothing of it.
func TestTreeDepthBoundHidesTheEdgesOfUndrawnMembers(t *testing.T) {
	model := `package Model {
	part def Body;
	part def Door;
	part def Car { part body : Body { part door : Door; } }
}
package Views {
	view doors { expose Model::Car; expose Model::Door; }
}`
	if got := treeEdgeLines(renderSource(t, "Views::doors", model)); !reflect.DeepEqual(got, []string{"body composition Model::Door: door"}) {
		t.Errorf("edges = %q, want the door drawn from the body it is nested in", got)
	}
	r, idx := loadSources(t, []string{"inline.sysml"}, [][]byte{[]byte(model)})
	r.treeDepthBound = 1
	rendering, err := r.Render(lookup(t, idx, "Views::doors"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := treeEdgeLines(rendering); len(got) != 0 {
		t.Errorf("edges = %q, want none: the door is not shown", got)
	}
}

// A bound written as an expression — which the parser diagnoses but keeps — is
// spelled with its operators when no source text is at hand, grouped where
// precedence would otherwise read it back differently, never as `?`.
func TestMultiplicityExpressionBoundsAreSpelledFromTheTree(t *testing.T) {
	name := func(text string) ast.Node {
		qn := &ast.QualifiedName{}
		qn.SetSingleton(ast.NameSegment{Text: text})
		return &ast.FeatureReference{Name: qn}
	}
	literal := func(value string) ast.Node { return &ast.LiteralInteger{Value: value} }
	op := func(kind ast.OperatorKind, operands ...ast.Node) ast.Node {
		return &ast.OperatorExpr{Operator: kind, Operands: operands}
	}
	cases := []struct {
		m    *ast.Multiplicity
		want string
	}{
		{&ast.Multiplicity{Lower: op(ast.OpAdd, name("capacity"), literal("1"))}, "[capacity + 1]"},
		{&ast.Multiplicity{Lower: literal("0"), Upper: op(ast.OpMul, op(ast.OpAdd, name("rows"), literal("1")), literal("2")), IsRange: true}, "[0..(rows + 1) * 2]"},
		{&ast.Multiplicity{Lower: op(ast.OpSub, name("n"), op(ast.OpSub, name("m"), literal("1")))}, "[n - (m - 1)]"},
		{&ast.Multiplicity{Lower: op(ast.OpSub, op(ast.OpSub, name("n"), name("m")), literal("1"))}, "[n - m - 1]"},
		{&ast.Multiplicity{Lower: op(ast.OpPow, name("n"), op(ast.OpPow, name("m"), literal("2")))}, "[n ** m ** 2]"},
		{&ast.Multiplicity{Lower: op(ast.OpPow, op(ast.OpPow, name("n"), name("m")), literal("2"))}, "[(n ** m) ** 2]"},
		{&ast.Multiplicity{Lower: op(ast.OpNeg, name("n"))}, "[-n]"},
		{&ast.Multiplicity{Lower: op(ast.OpNeg, op(ast.OpAdd, name("n"), literal("1")))}, "[-(n + 1)]"},
		{&ast.Multiplicity{Lower: literal("1"), Upper: &ast.LiteralInfinity{}, IsRange: true}, "[1..*]"},
	}
	r := &Renderer{}
	for _, c := range cases {
		if got := r.multiplicityText("", c.m); got != c.want {
			t.Errorf("multiplicityText = %q, want %q", got, c.want)
		}
	}
}

// A connection def whose two ends are typed by drawn definitions is the
// association a block definition diagram draws as a line between their
// boxes, labelled with its ends and its own name, not a box of its own. One
// that carries a member of its own, as an association block does, or whose
// end type is not drawn stays a box.
func TestTreeDrawsAConnectionDefAsALineBetweenItsEndTypes(t *testing.T) {
	rendering := render(t, "tree-associations.sysml", "FleetViews::structure")
	assertRenderingEdgeEndpoints(t, rendering)
	var names []string
	var visit func(nodes []*Node)
	visit = func(nodes []*Node) {
		for _, node := range nodes {
			names = append(names, node.Name)
			visit(node.Children)
		}
	}
	visit(rendering.Roots)
	for _, boxed := range []string{"Fleet::WheelToCar", "Fleet::Tows"} {
		if slices.Contains(names, boxed) {
			t.Errorf("%s is drawn as a box; nodes = %q", boxed, names)
		}
	}
	for _, boxed := range []string{"Fleet::Pairing", "Fleet::Ferries", "Fleet::Keyed", "Fleet::Towing"} {
		if !slices.Contains(names, boxed) {
			t.Errorf("%s is not drawn as a box; nodes = %q", boxed, names)
		}
	}
	want := []string{
		"Fleet::Wheel association Fleet::Car: WheelToCar: wheels[4] / car",
		"Fleet::Truck composition Fleet::Hitch: HitchToTruck: hitch / truck",
		"Fleet::Car association Fleet::Trailer: Tows: tower / trailer[0..1]",
		"Fleet::Pairing reference Fleet::Car: a",
		"Fleet::Pairing reference Fleet::Trailer: b",
		"Fleet::Ferries reference Fleet::Car: car",
		"Fleet::Keyed reference Fleet::Car: a",
		"Fleet::Keyed reference Fleet::Trailer: b",
	}
	got := treeEdgeLines(rendering)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("edges = %q, want %q", got, want)
	}
}

// The line a connection def is drawn as wears the def's Style and Route and
// carries its Notes, anchored to the line instead of to the box it no longer is.
func TestTreeAssociationLineKeepsTheDefsPresentation(t *testing.T) {
	rendering := renderSource(t, "Views::styled", `package Model {
	part def Wheel;
	part def Car;
	connection def WheelToCar { end wheel : Wheel; end car : Car; }
}
package Views {
	private import DiagramLayout::*;
	view styled {
		expose Model::Car;
		expose Model::Wheel;
		expose Model::WheelToCar;
		metadata Style about Model::WheelToCar { line = "#00FF00"; }
		metadata Route about Model::WheelToCar { points = (10, 20, 30, 40); }
		metadata Note about Model::WheelToCar { text = "four of them"; x = 10; y = 20; }
	}
}`)
	if len(rendering.Roots) != 2 {
		t.Fatalf("roots = %+v, want Wheel and Car alone", rendering.Roots)
	}
	if len(rendering.Edges) != 1 || rendering.Edges[0].Kind != EdgeAssociation {
		t.Fatalf("edges = %+v, want the one connection line", rendering.Edges)
	}
	edge := rendering.Edges[0]
	if edge.Style == nil || edge.Style.Line != "#00FF00" {
		t.Errorf("edge style = %+v, want the def's line", edge.Style)
	}
	if len(edge.Route) != 2 {
		t.Errorf("edge route = %+v, want the def's two points", edge.Route)
	}
	if len(rendering.Notes) != 1 || rendering.Notes[0].Anchor != "" || rendering.Notes[0].EdgeFrom != edge.From || rendering.Notes[0].EdgeTo != edge.To {
		t.Errorf("notes = %+v, want the one note anchored to the line", rendering.Notes)
	}
}

// An end crossing a composite part the tree already draws as a composition
// from its owner takes that edge over: the association is one line, keeping
// the composition's diamond and the part's name, wearing the def's Style and
// following its Route, with the def's Notes anchored to it.
func TestTreeAssociationCrossingACompositePartIsItsCompositionEdge(t *testing.T) {
	rendering := renderSource(t, "Diagrams::bdd", `package Model {
	part def Wheel;
	part def Car {
		part wheels : Wheel[4];
	}
	connection def WheelToCar {
		end wheels : Wheel crosses car.wheels;
		end car : Car;
	}
}
package Diagrams {
	private import DiagramLayout::*;
	view bdd {
		expose Model::Car;
		expose Model::Wheel;
		expose Model::WheelToCar;
		metadata Style about Model::WheelToCar { line = "#00FF00"; }
		metadata Route about Model::WheelToCar { points = (10, 20, 30, 40); }
		metadata Note about Model::WheelToCar { text = "four of them"; x = 10; y = 20; }
		render Views::asTreeDiagram;
	}
}`)
	if got, want := treeEdgeLines(rendering), []string{"Model::Car composition Model::Wheel: WheelToCar: wheels[4] / car"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("edges = %q, want %q", got, want)
	}
	edge := rendering.Edges[0]
	if edge.Style == nil || edge.Style.Line != "#00FF00" {
		t.Errorf("edge style = %+v, want the def's line", edge.Style)
	}
	// The def's route runs from Wheel to Car; the composition runs from Car.
	if want := []Point{{30, 40}, {10, 20}}; !reflect.DeepEqual(edge.Route, want) {
		t.Errorf("edge route = %v, want the def's reversed %v", edge.Route, want)
	}
	if len(rendering.Notes) != 1 || rendering.Notes[0].Anchor != "" || rendering.Notes[0].EdgeFrom != edge.From || rendering.Notes[0].EdgeTo != edge.To {
		t.Errorf("notes = %+v, want the one note anchored to the edge", rendering.Notes)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	var lines []string
	for _, line := range strings.Split(dot, "\n") {
		if strings.Contains(line, `label="WheelToCar`) {
			lines = append(lines, line)
		}
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "dir=back, arrowtail=diamond") || strings.Contains(lines[0], "penwidth=3") {
		t.Errorf("DOT draws the association as other than one composition edge:\n%s", dot)
	}
}

// An association line is a plain line, as a block definition diagram draws
// one, not the heavy connector of an interconnection diagram.
func TestTreeAssociationLineIsPlain(t *testing.T) {
	rendering := render(t, "tree-associations.sysml", "FleetViews::structure")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if strings.Contains(dot, "penwidth=3") {
		t.Errorf("DOT draws an association as a heavy connector:\n%s", dot)
	}
	if want := `[label="Tows: tower / trailer[0..1]", dir=none];`; !strings.Contains(dot, want) {
		t.Errorf("DOT lacks the plain line %s:\n%s", want, dot)
	}
}

// A def whose second end crosses the composite part keeps the direction of
// its route, which already runs from the part's owner.
func TestTreeAssociationCrossingACompositePartFromItsSecondEndKeepsItsRoute(t *testing.T) {
	rendering := renderSource(t, "Diagrams::bdd", `package Model {
	part def Wheel;
	part def Car {
		part wheels : Wheel[4];
	}
	connection def CarToWheel {
		end car : Car;
		end wheels : Wheel crosses car.wheels;
	}
}
package Diagrams {
	private import DiagramLayout::*;
	view bdd {
		expose Model::Car;
		expose Model::Wheel;
		expose Model::CarToWheel;
		metadata Route about Model::CarToWheel { points = (10, 20, 30, 40); }
		render Views::asTreeDiagram;
	}
}`)
	if got, want := treeEdgeLines(rendering), []string{"Model::Car composition Model::Wheel: CarToWheel: car / wheels[4]"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("edges = %q, want %q", got, want)
	}
	if want := []Point{{10, 20}, {30, 40}}; !reflect.DeepEqual(rendering.Edges[0].Route, want) {
		t.Errorf("edge route = %v, want the def's %v", rendering.Edges[0].Route, want)
	}
}

// An association line stands only for the edge from the crossed part to the
// end's type; a part typed by more definitions keeps its edges to the others.
func TestTreeAssociationCrossingAMultiplyTypedPartKeepsItsOtherEdges(t *testing.T) {
	rendering := renderSource(t, "Diagrams::bdd", `package Model {
	part def Wheel;
	part def Electric;
	part def Car {
		part hybrid : Wheel, Electric;
	}
	connection def WheelToCar {
		end wheels : Wheel crosses car.hybrid;
		end car : Car;
	}
}
package Diagrams {
	view bdd {
		expose Model::Car;
		expose Model::Wheel;
		expose Model::Electric;
		expose Model::WheelToCar;
		render Views::asTreeDiagram;
	}
}`)
	want := []string{
		"Model::Car composition Model::Wheel: WheelToCar: hybrid / car",
		"Model::Car composition Model::Electric: hybrid",
	}
	if got := treeEdgeLines(rendering); !reflect.DeepEqual(got, want) {
		t.Errorf("edges =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}
