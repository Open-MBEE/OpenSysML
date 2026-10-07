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

// An exposed analysis def is a node of an interconnection rendering, and a feature
// whose value names a drawn feature is a binding edge between the two, steered by
// the Route about the feature. A value of a feature the rendering does not draw
// binds to the nearest node drawn, the subject; a literal draws no edge.
func TestFeatureValuesAreBindingEdges(t *testing.T) {
	r := renderSource(t, "Model::both", valueBindingModel)
	if len(r.Notices) != 0 {
		t.Errorf("notices = %v, want none", r.Notices)
	}
	analysis := nodeByName(r, "Model::'Sensor Monte Carlo'")
	x, observed, hidden, analysed, mean := nodeByName(r, "Model::Sensor::x"), nodeByName(r, "observed"), nodeByName(r, "hidden"), nodeByName(r, "analysed"), nodeByName(r, "mean")
	if analysis == nil || x == nil || observed == nil || hidden == nil || analysed == nil || mean == nil {
		t.Fatalf("nodes = %v, want the analysis def, x and the analysis members", nodeNames(r.Roots))
	}
	if analysis.Kind != "analysis def" || analysis.Geometry == nil || analysis.Geometry.X != 70 || analysis.Geometry.Width != 81 {
		t.Errorf("analysis node = %s %+v, want an analysis def at its Layout", analysis.Kind, analysis.Geometry)
	}
	edges := bindingEdges(r)
	if len(edges) != 3 {
		t.Fatalf("binding edges = %+v, want observed = analysed.x, hidden = analysed.y and mean = observed", edges)
	}
	want := map[[2]string]bool{{observed.ID, x.ID}: true, {hidden.ID, analysed.ID}: true, {mean.ID, observed.ID}: true}
	for _, e := range edges {
		if !want[[2]string{e.From, e.To}] {
			t.Errorf("edge %s -> %s is none of the bindings wanted", e.From, e.To)
		}
		if e.Label != "binding" {
			t.Errorf("edge %s -> %s labelled %q, want binding", e.From, e.To, e.Label)
		}
		if e.From == observed.ID && e.To == x.ID && (len(e.Route) != 2 || e.Route[0].X != 105 || e.Route[1].Y != 98) {
			t.Errorf("observed binding route = %v, want the Route stated", e.Route)
		}
	}
	if text := r.Text(); strings.Count(text, "==") != 3 {
		t.Errorf("text draws %d bindings, want three:\n%s", strings.Count(text, "=="), text)
	}
	dot, err := r.DOT()
	if err != nil {
		t.Fatal(err)
	}
	if want := `"` + observed.ID + `" -> "` + x.ID + `" [label="binding", arrowhead=none, pos="105,-82 105,-82 105,-98 105,-98"`; !strings.Contains(dot, want) {
		t.Errorf("DOT lacks the routed binding %q:\n%s", want, dot)
	}
}

// With the analysed value not exposed, the binding of observed ends at the subject
// node, the nearest the rendering draws of what the value names.
func TestFeatureValueBindsToTheNearestDrawnNode(t *testing.T) {
	r := renderSource(t, "Model::analysisOnly", valueBindingModel)
	observed, analysed := nodeByName(r, "observed"), nodeByName(r, "analysed")
	if observed == nil || analysed == nil {
		t.Fatalf("nodes = %v, want observed and analysed", nodeNames(r.Roots))
	}
	var found bool
	for _, e := range bindingEdges(r) {
		if e.From == observed.ID && e.To == analysed.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("edges = %+v, want observed bound to analysed", r.Edges)
	}
}
