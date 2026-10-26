package view

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
		if text := edge.text(); text != "" {
			line += ": " + text
		}
		lines = append(lines, line)
	}
	return lines
}

// A tree draws the relationships between the elements it has nodes for: the
// membership of each structural usage drawn under its owner, declared or
// inherited — a composition from the owner to a part's node, a reference to a
// `ref` part's or an attribute's, the multiplicity at the usage's end — each
// usage's typing to its drawn definition, and a specialization, redefinition
// or subsetting to its general. A definition nested in its owner draws no
// edge from it, nor is an edge drawn to an element the tree does not draw or
// a type that does not resolve.
func TestTreeDrawsTheRelationshipsBetweenItsNodes(t *testing.T) {
	rendering := render(t, "tree-edges.sysml", "FleetViews::structure")
	assertRenderingEdgeEndpoints(t, rendering)
	want := []string{
		"Fleet::Vehicle composition chassis",
		"Fleet::Vehicle composition wheels: [4]",
		"Fleet::Vehicle reference driver: [0..1]",
		"Fleet::Vehicle reference total",
		"Fleet::Vehicle composition cabin",
		"Fleet::Vehicle composition radio",
		"Fleet::Vehicle composition trailer",
		"chassis typing Fleet::Chassis",
		"chassis reference mass",
		"wheels typing Fleet::Wheel",
		"driver typing Fleet::Driver",
		"total typing Fleet::Mass",
		"cabin typing Cabin",
		"Fleet::SportsCar specialization Fleet::Vehicle",
		"Fleet::SportsCar composition chassis",
		"Fleet::SportsCar composition spare",
		"Fleet::SportsCar composition wheels: [4]",
		"Fleet::SportsCar reference driver: [0..1]",
		"Fleet::SportsCar reference total",
		"Fleet::SportsCar composition cabin",
		"Fleet::SportsCar composition radio",
		"Fleet::SportsCar composition trailer",
		"chassis specialization chassis: redefines",
		"chassis typing Fleet::LightChassis",
		"chassis reference mass",
		"spare typing Fleet::Wheel",
		"spare specialization wheels: subsets",
		"Fleet::Chassis reference mass",
		"mass typing Fleet::Mass",
		"Fleet::LightChassis specialization Fleet::Chassis",
		"Fleet::LightChassis reference mass",
		"mass specialization mass: redefines",
		"Fleet::fleetCar typing Fleet::Vehicle",
		"Fleet::fleetCar composition chassis",
		"Fleet::fleetCar composition wheels: [4]",
		"Fleet::fleetCar reference driver: [0..1]",
		"Fleet::fleetCar reference total",
		"Fleet::fleetCar composition cabin",
		"Fleet::fleetCar composition radio",
		"Fleet::fleetCar composition trailer",
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

// A membership edge stands for the usage it leads to, so the Route about that
// usage steers it and a layout check sees the usage drawn as an edge; the
// usage's typing, like a specialization, has no member of its own and is left
// to the client to route.
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
			routed = append(routed, edge.Kind.String()+" "+edge.ToLabel)
			if want := []Point{{100, 200}, {300, 200}}; !reflect.DeepEqual(edge.Route, want) {
				t.Errorf("route of %s = %v, want %v", edge.ToLabel, edge.Route, want)
			}
		}
	}
	if !reflect.DeepEqual(routed, []string{"composition [4]"}) {
		t.Errorf("routed edges = %v, want the wheels membership alone", routed)
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

// A membership edge originates at the usage it stands for; a specialization,
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
		"Fleet::Vehicle composition wheels: [4]":            "part wheels[4] : Wheel;",
		"Fleet::Vehicle reference driver: [0..1]":           "ref part driver[0..1] : Driver;",
		"wheels typing Fleet::Wheel":                        "Wheel",
		"Fleet::SportsCar specialization Fleet::Vehicle":    "Vehicle",
		"chassis specialization chassis: redefines":         "chassis",
		"spare specialization wheels: subsets":              "wheels",
		"Fleet::LightChassis specialization Fleet::Chassis": "Chassis",
		"Fleet::fleetCar typing Fleet::Vehicle":             "Vehicle",
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
	want := []string{
		"Vehicles::Vehicle composition engine",
		"Vehicles::Vehicle composition wheels: [4]",
		"engine typing Vehicles::Engine",
		"engine reference power",
		"engine composition cylinder: [4]",
		"Vehicles::Engine reference power",
		"Vehicles::Engine composition cylinder: [4]",
	}
	if got := treeEdgeLines(rendering); !reflect.DeepEqual(got, want) {
		t.Errorf("edges = %v, want %v", got, want)
	}
}

// The text form lists a tree's edges as the relationships they are, in the
// class-diagram arrows the general graphs use.
func TestTreeTextListsRelationships(t *testing.T) {
	text := render(t, "tree-edges.sysml", "FleetViews::structure").textWith(fullNotation)
	for _, want := range []string{
		"\nrelationships:\n",
		"  Fleet::Vehicle *-- wheels: [4]\n",
		"  Fleet::Vehicle o-- driver: [0..1]\n",
		"  wheels ..|> Fleet::Wheel\n",
		"  Fleet::SportsCar --|> Fleet::Vehicle\n",
		"  Fleet::fleetCar ..|> Fleet::Vehicle\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
}

// A usage's Style colours the membership edge standing for it as it does the
// usage's node, not its typing edge; its Note stays anchored to the node
// alone, not repeated on the edge.
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
	if len(rendering.Edges) != 2 || rendering.Edges[0].Kind != EdgeComposition || rendering.Edges[0].Style == nil || rendering.Edges[0].Style.Line != "#FF0000" {
		t.Fatalf("edges = %+v, want the wheels' composition styled with their line, then their typing", rendering.Edges)
	}
	if rendering.Edges[1].Kind != EdgeTyping || rendering.Edges[1].Style != nil {
		t.Errorf("typing edge = %+v, want it undressed", rendering.Edges[1])
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
		if edge.ToLabel != "" {
			labels = append(labels, edge.ToLabel)
		}
	}
	if want := []string{"[axles]", "[0..rows]"}; !reflect.DeepEqual(labels, want) {
		t.Errorf("labels = %q, want %q", labels, want)
	}
}

// Two anonymous usages of one type are two memberships, each drawn with its
// own typing.
func TestTreeDrawsEachAnonymousUsage(t *testing.T) {
	rendering := renderSource(t, "Views::pair", `package Model {
	part def B;
	part def A { part : B; part : B; }
}
package Views {
	view pair { expose Model::A; expose Model::B; }
}`)
	want := []string{"Model::A composition ", "Model::A composition ", " typing Model::B", " typing Model::B"}
	if got := treeEdgeLines(rendering); !reflect.DeepEqual(got, want) {
		t.Errorf("edges = %q, want both anonymous memberships and typings", got)
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
	want := []string{"Model::Car composition body", "body composition door", "door typing Model::Door"}
	if got := treeEdgeLines(renderSource(t, "Views::doors", model)); !reflect.DeepEqual(got, want) {
		t.Errorf("edges = %q, want the door's membership in the body and its typing", got)
	}
	r, idx := loadSources(t, []string{"inline.sysml"}, [][]byte{[]byte(model)})
	r.treeDepthBound = 1
	rendering, err := r.Render(lookup(t, idx, "Views::doors"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := treeEdgeLines(rendering); !reflect.DeepEqual(got, []string{"Model::Car composition body"}) {
		t.Errorf("edges = %q, want the body's membership alone: the door is not shown", got)
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
		"Fleet::Wheel association Fleet::Car: wheels[4] WheelToCar car",
		"Fleet::Truck composition Fleet::Hitch: truck HitchToTruck hitch",
		"Fleet::Car association Fleet::Trailer: tower Tows trailer[0..1]",
		"Fleet::Car reference wheels: [4]",
		"wheels reference car",
		"Fleet::Wheel reference car",
		"car reference wheels: [4]",
		"Fleet::Truck composition hitch",
		"Fleet::Pairing reference a",
		"Fleet::Pairing reference b",
		"Fleet::Pairing reference tag",
		"a typing Fleet::Car",
		"a reference wheels: [4]",
		"b typing Fleet::Trailer",
		"Fleet::Ferries reference boat",
		"Fleet::Ferries reference car",
		"car typing Fleet::Car",
		"car reference wheels: [4]",
		"Fleet::Keyed reference a",
		"Fleet::Keyed reference b",
		"a typing Fleet::Car",
		"a reference key",
		"a reference wheels: [4]",
		"b typing Fleet::Trailer",
		"Fleet::Towing reference tower",
		"Fleet::Towing reference trailer: [0..1]",
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

// An end crossing a composite part stands for the part's typing by the end's
// type: the association is one line with the composition's diamond and the
// part's name, wearing the def's Style and following its Route, with the
// def's Notes anchored to it; the part's node keeps its membership edge alone.
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
	if got, want := treeEdgeLines(rendering), []string{"Model::Car composition Model::Wheel: car WheelToCar wheels[4]", "Model::Car composition wheels: [4]"}; !reflect.DeepEqual(got, want) {
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
	if want := `[label="Tows", taillabel="tower", headlabel="trailer", dir=none];`; !strings.Contains(dot, want) {
		t.Errorf("DOT lacks the plain line, its ends named without the multiplicity left out by default %s:\n%s", want, dot)
	}
	full, err := rendering.DOTWith(fullNotation)
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if want := `[label="Tows", taillabel="tower", headlabel="trailer[0..1]", dir=none];`; !strings.Contains(full, want) {
		t.Errorf("DOT with every element lacks the plain line, the def's name along it %s:\n%s", want, full)
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
	if got, want := treeEdgeLines(rendering), []string{"Model::Car composition Model::Wheel: car CarToWheel wheels[4]", "Model::Car composition wheels: [4]"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("edges = %q, want %q", got, want)
	}
	if want := []Point{{10, 20}, {30, 40}}; !reflect.DeepEqual(rendering.Edges[0].Route, want) {
		t.Errorf("edge route = %v, want the def's %v", rendering.Edges[0].Route, want)
	}
}

// An association line stands only for the crossed part's typing by the end's
// type; a part typed by more definitions keeps its typing edges to the others,
// and its membership edge.
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
		"Model::Car composition Model::Wheel: car WheelToCar hybrid",
		"Model::Car composition hybrid",
		"hybrid typing Model::Electric",
	}
	if got := treeEdgeLines(rendering); !reflect.DeepEqual(got, want) {
		t.Errorf("edges =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// The membership edge from an owner to a usage drawn under it takes its
// diamond from the model's Feature::isComposite: filled for a composite part,
// hollow for a `ref` part, a port nested in a part definition, an end feature,
// an enumeration usage and a usage subsetting another by `references`.
func TestTreeMembershipDiamondFollowsTheModel(t *testing.T) {
	rendering := renderSource(t, "Views::members", `package Model {
	part def Wheel;
	port def Plug;
	enum def Colour { red; }
	part def Car {
		part wheels[4] : Wheel;
		ref part spare : Wheel;
		port plug : Plug;
		enum paint : Colour;
		part link references wheels;
	}
	part def Hook;
	connection def Pairing {
		end a : Hook;
		end b : Hook;
		attribute tag : ScalarValues::String;
	}
}
package Views {
	view members { expose Model::Car; expose Model::Pairing; }
}`)
	want := []string{
		"Model::Car composition wheels: [4]",
		"Model::Car reference spare",
		"Model::Car reference plug",
		"Model::Car reference paint",
		"Model::Car reference link",
		"Model::Pairing reference a",
		"Model::Pairing reference b",
		"Model::Pairing reference tag",
	}
	if got := treeEdgeLines(rendering); !reflect.DeepEqual(got, want) {
		t.Errorf("edges =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// A usage drawn as a node is joined to its owner by its membership and to its
// definition by its typing; the same usage listed in a compartment row of its
// owner folds the two into the owner's edge to the definition, labelled with
// the usage's name and multiplicity, its diamond read the same way. This is
// the model of the report on the port-membership endpoint and compositeness.
func TestTreeMembershipEndsAtTheUsageDrawnAndAtTheDefinitionListed(t *testing.T) {
	r, idx := loadFixture(t, "feature-membership.sysml")
	exposed := []*symbols.Symbol{lookup(t, idx, "A::APart"), lookup(t, idx, "A::APort"), lookup(t, idx, "A::part1"), lookup(t, idx, "A::part2")}
	rendering, err := r.RenderExposed(exposed, KindTree, "")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := []string{
		"A::APart reference aPort",
		"aPort typing A::APort",
		"A::part1 typing A::APart",
		"A::part1 reference aPort",
		"A::part2 typing A::APart",
		"A::part2 reference aPort",
	}
	if got := treeEdgeLines(rendering); !reflect.DeepEqual(got, want) {
		t.Errorf("edges =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{`"n0" -> "n1" [dir=back, arrowtail=odiamond];`, `"n1" -> "n2" [arrowhead=empty, style=dashed];`} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	if strings.Contains(dot, `"n0" -> "n2"`) || strings.Contains(dot, "arrowhead=none") {
		t.Errorf("DOT draws the owner's edge to the port definition, or a bare containment line, beside the port's node:\n%s", dot)
	}
	folded := foldCompartments(rendering.showing(AllShown()), CompartmentsMost)
	want = []string{"A::APart reference A::APort: aPort", "A::part1 typing A::APart", "A::part2 typing A::APart"}
	if got := treeEdgeLines(folded); !reflect.DeepEqual(got, want) {
		t.Errorf("folded edges = %q, want %q: the listed port's membership and typing fold into the owner's edge to the definition", got, want)
	}
}

// The text form lists no edge at a member a Layout hides: the hidden member's
// membership and typing fold into its owner's edge to the drawn definition,
// labelled with the member, as the compartment foldings draw them.
func TestTreeTextFoldsTheEdgesAtHiddenMembers(t *testing.T) {
	rendering := renderSource(t, "Views::collapsed", `package Model {
	part def Lens;
	part def Imaging {
		part lens : Lens;
		ref part spare : Lens;
		port data;
	}
}
package Views {
	private import DiagramLayout::*;
	view collapsed {
		expose Model::Imaging;
		expose Model::Lens;
		metadata Layout about Model::Imaging { x = 10; y = 10; width = 200; height = 33; showParts = false; showReferences = false; showPorts = false; }
		metadata Layout about Model::Lens { x = 10; y = 100; width = 200; height = 33; }
	}
}`)
	text := rendering.Text()
	for _, want := range []string{"  Imaging *-- Lens: lens\n", "  Imaging o-- Lens: spare\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasSuffix(line, "-- ") || strings.Contains(line, "data") {
			t.Errorf("text lists an edge at a hidden member: %q\n%s", line, text)
		}
	}
}

// A member hidden with the nodes under it folds its own membership alone: a
// typed node under a hidden member draws no edge from the member's owner,
// which does not own it.
func TestTreeTextDrawsNoEdgeForTheNodesUnderAHiddenMember(t *testing.T) {
	rendering := renderSource(t, "Views::collapsed", `package Model {
	part def Lens;
	part def Assembly { part lens : Lens; }
	part def Imaging { part assembly : Assembly; }
}
package Views {
	private import DiagramLayout::*;
	view collapsed {
		expose Model::Imaging;
		expose Model::Lens;
		metadata Layout about Model::Imaging { x = 10; y = 10; width = 200; height = 33; showParts = false; }
		metadata Layout about Model::Lens { x = 10; y = 100; width = 200; height = 33; }
	}
}`)
	if text := rendering.Text(); strings.Contains(text, "relationships") {
		t.Errorf("text lists an edge for the hidden assembly's lens, which Imaging does not own:\n%s", text)
	}
}

// A tree showing no compartments leaves an unplaced member undrawn; its
// membership and typing still fold into the owner's labelled edge to the
// definition, the one way the relationship is shown, and a Route about the
// member pins that edge.
func TestTreeWithoutCompartmentsKeepsTheEdgeOfAnUndrawnMember(t *testing.T) {
	rendering := renderSource(t, "Bare::bdd", `package Model {
	part def Wheel;
	part def Car { part wheels : Wheel; }
}
package Bare {
	rendering bareTree :> Views::asTreeDiagram {
		attribute showCompartments : ScalarValues::Boolean default = false;
	}
	view bdd {
		expose Model::Car;
		expose Model::Wheel;
		metadata DiagramLayout::Route about Model::Car::wheels { points = (10, 20, 200, 20); }
		render bareTree;
	}
}`)
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{"// layout: neato -n2\n", `"n0" -> "n2" [headlabel="wheels", dir=back, arrowtail=diamond, pos="s,10,-20 20,-20 20,-20 200,-20 200,-20"`, "// not represented: 1 node(s) without a position, left undrawn\n"} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
}

// A Route about a usage, with no Layout anywhere, positions the drawing as it
// did when it steered the owner's edge to the definition: the usage folds
// into a row of its owner, and DOT pins the labelled edge along the route.
func TestTreeRouteAboutAUsagePositionsTheDrawing(t *testing.T) {
	rendering := renderSource(t, "Views::bdd", `package Model {
	part def Wheel;
	part def Car { part wheels : Wheel; }
}
package Views {
	view bdd {
		expose Model::Car;
		expose Model::Wheel;
		metadata DiagramLayout::Route about Model::Car::wheels { points = (10, 20, 200, 20); }
	}
}`)
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{"// layout: neato -n2\n", `"n0" -> "n2" [headlabel="wheels", dir=back, arrowtail=diamond, pos="s,10,-20 20,-20 20,-20 200,-20 200,-20"`} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	if strings.Contains(dot, `"n0" -> "n1"`) {
		t.Errorf("DOT draws the routed membership to the wheels' node instead of folding it:\n%s", dot)
	}
}
