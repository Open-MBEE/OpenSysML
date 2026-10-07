package view

import (
	"strings"
	"testing"
)

// valueBindingModel has an analysis whose observed value is bound to the analysed
// part's x: the migrated form of a Monte Carlo analysis, drawn beside its subject's
// values in a parametric view.
const valueBindingModel = `package Model {
	part def Sensor {
		attribute x : ScalarValues::Real;
		attribute y : ScalarValues::Real;
	}
	analysis def 'Sensor Monte Carlo' {
		subject analysed : Sensor;
		attribute observed : ScalarValues::Real = analysed.x;
		attribute hidden : ScalarValues::Real = analysed.y;
		attribute spec : ScalarValues::Real = 10.0;
		return mean : ScalarValues::Real = observed;
	}
	view both {
		expose Sensor::x;
		expose 'Sensor Monte Carlo';
		metadata DiagramLayout::Layout about 'Sensor Monte Carlo' { x = 70; y = 56; width = 81; height = 26; }
		metadata DiagramLayout::Route about 'Sensor Monte Carlo'::observed { points = (105, 82, 105, 98); }
		render Views::asInterconnectionDiagram;
	}
	view analysisOnly {
		expose 'Sensor Monte Carlo';
		render Views::asInterconnectionDiagram;
	}
}`

func bindingEdges(r *Rendering) []Edge {
	var edges []Edge
	for _, e := range r.Edges {
		if e.Kind == EdgeBinding {
			edges = append(edges, e)
		}
	}
	return edges
}

func nodeByName(r *Rendering, name string) *Node {
	for _, node := range everyNode(r.Roots) {
		if node.Name == name {
			return node
		}
	}
	return nil
}

// An exposed analysis def is a node of an interconnection rendering with its
// subject, parameters and attributes as pins on its border, not nodes inside it;
// a pin whose value names a drawn feature is a binding edge from the pin to the
// feature, steered by the Route about the pin's feature. A pin bound to another
// of the same node (mean = observed, hidden = analysed.y) is the node's own
// wiring and draws no edge; nor does a literal.
func TestFeatureValuesAreBindingEdges(t *testing.T) {
	r := renderSource(t, "Model::both", valueBindingModel)
	if len(r.Notices) != 0 {
		t.Errorf("notices = %v, want none", r.Notices)
	}
	analysis, x := nodeByName(r, "Model::'Sensor Monte Carlo'"), nodeByName(r, "Model::Sensor::x")
	if analysis == nil || x == nil {
		t.Fatalf("nodes = %v, want the analysis def and x", nodeNames(r.Roots))
	}
	if len(nodeNames(r.Roots)) != 2 || len(analysis.Children) != 0 {
		t.Errorf("nodes = %v with %d nested in the analysis def, want the two alone", nodeNames(r.Roots), len(analysis.Children))
	}
	if analysis.Kind != "analysis def" || analysis.Geometry == nil || analysis.Geometry.X != 70 || analysis.Geometry.Width != 81 {
		t.Errorf("analysis node = %s %+v, want an analysis def at its Layout", analysis.Kind, analysis.Geometry)
	}
	wantPins := map[string]PortDirection{"analysed": PortUndirected, "observed": PortUndirected, "hidden": PortUndirected, "spec": PortUndirected, "mean": PortOut}
	if len(analysis.Ports) != len(wantPins) {
		t.Errorf("analysis pins = %+v, want %v", analysis.Ports, wantPins)
	}
	for name, direction := range wantPins {
		if pin := pinNamed(analysis, name); pin == nil || pin.Direction != direction || pin.Type != "ScalarValues::Real" && name != "analysed" {
			t.Errorf("pin %s = %+v, want direction %v typed ScalarValues::Real", name, pin, direction)
		}
	}
	observed := pinNamed(analysis, "observed")
	edges := bindingEdges(r)
	if len(edges) != 1 {
		t.Fatalf("binding edges = %+v, want observed = analysed.x alone", edges)
	}
	e := edges[0]
	if e.From != analysis.ID || e.FromPort != observed.ID || e.To != x.ID || e.ToPort != "" || e.Label != "binding" {
		t.Errorf("edge = %+v, want a binding from pin %s of %s to %s", e, observed.ID, analysis.ID, x.ID)
	}
	if len(e.Route) != 2 || e.Route[0].X != 105 || e.Route[1].Y != 98 {
		t.Errorf("observed binding route = %v, want the Route stated", e.Route)
	}
	if text := r.Text(); strings.Count(text, "==") != 1 {
		t.Errorf("text draws %d bindings, want one:\n%s", strings.Count(text, "=="), text)
	}
	dot, err := r.DOT()
	if err != nil {
		t.Fatal(err)
	}
	if want := `"` + observed.ID + `" -> "` + x.ID + `" [label="binding", arrowhead=none, pos="105,-82 105,-82 105,-98 105,-98"`; !strings.Contains(dot, want) {
		t.Errorf("DOT lacks the routed binding %q:\n%s", want, dot)
	}
}

// With the analysed value not exposed, observed's value names a feature of the
// subject, a pin of the analysis node itself, so no edge is drawn: the analysis
// def is one node with its pins and nothing else.
func TestFeatureValueToTheNodesOwnPinDrawsNoEdge(t *testing.T) {
	r := renderSource(t, "Model::analysisOnly", valueBindingModel)
	analysis := nodeByName(r, "Model::'Sensor Monte Carlo'")
	if analysis == nil || len(nodeNames(r.Roots)) != 1 {
		t.Fatalf("nodes = %v, want the analysis def alone", nodeNames(r.Roots))
	}
	if pinNamed(analysis, "observed") == nil || pinNamed(analysis, "analysed") == nil {
		t.Errorf("pins = %+v, want observed and analysed", analysis.Ports)
	}
	if len(r.Edges) != 0 {
		t.Errorf("edges = %+v, want none", r.Edges)
	}
}
