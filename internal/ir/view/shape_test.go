package view

import (
	"strings"
	"testing"
)

// A render reports each node's shape without reading a written form: a case is
// a stadium, its actor a stick figure, a requirement a rectangle.
func TestNodeDataCarriesNotationShape(t *testing.T) {
	renderer, index := loadFixture(t, "shape-nodes.sysml")
	rendering, err := renderer.Render(lookup(t, index, "ShapeExamples::shapes"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if rendering.Kind != KindMixed {
		t.Fatalf("kind = %s, want mixed", rendering.Kind)
	}
	byKind := map[Shape][]string{}
	for _, node := range rendering.Data().Nodes {
		byKind[node.Shape] = append(byKind[node.Shape], node.Kind+" "+node.Name)
	}
	for _, want := range []struct {
		shape Shape
		label string
	}{
		{ShapeStadium, "case def Inspection"},
		{ShapeActor, "actor driver"},
		{ShapeRectangle, "requirement def Ready"},
	} {
		found := false
		for _, label := range byKind[want.shape] {
			if label == want.label {
				found = true
			}
		}
		if !found {
			t.Errorf("shape %q not reported for %q; nodes = %v", want.shape, want.label, byKind)
		}
	}
}

// These cases pin the derived shape against the written Mermaid shape for each
// kind; written goldens stay byte-identical because the field is additive.
func TestNodeShapeMatchesWrittenNotation(t *testing.T) {
	labels := labeller{shown: AllShown()}
	cases := []struct {
		kind    Kind
		node    NodeData
		shape   Shape
		mermaid string
	}{
		{KindCase, NodeData{Kind: "case def"}, ShapeStadium, "(["},
		{KindCase, NodeData{Kind: "actor"}, ShapeActor, "["},
		{KindTree, NodeData{Kind: "actor"}, ShapeRounded, "("},
		{KindInterconnection, NodeData{Kind: "actor"}, ShapeRounded, "("},
		{KindCase, NodeData{Kind: "subject"}, ShapeRectangle, "["},
		{KindTree, NodeData{Kind: "subject"}, ShapeRounded, "("},
		{KindInterconnection, NodeData{Kind: "subject"}, ShapeRounded, "("},
		{KindRequirement, NodeData{Kind: "requirement def"}, ShapeRectangle, "["},
		{KindRequirement, NodeData{Kind: "requirement"}, ShapeRounded, "("},
		{KindTree, NodeData{Kind: "case def"}, ShapeRectangle, "["},
		{KindCase, NodeData{Kind: "objective"}, ShapeNote, "@{ shape: notch-rect,"},
		{KindTree, NodeData{Kind: "objective"}, ShapeRounded, "("},
		{KindInterconnection, NodeData{Kind: "objective"}, ShapeRounded, "("},
		{KindAction, NodeData{Kind: "decision"}, ShapeDiamond, "{"},
		{KindAction, NodeData{Kind: "fork"}, ShapeBar, "@{ shape: fork,"},
		{KindState, NodeData{Kind: "final"}, ShapeRing, "@{ shape: fr-circ,"},
		{KindInterconnection, NodeData{Kind: "part"}, ShapeRounded, "("},
		{KindInterconnection, NodeData{Kind: "part def"}, ShapeRectangle, "["},
		{KindTree, NodeData{Kind: "part package"}, ShapeRectangle, "["},
		{KindInterconnection, NodeData{Kind: "part", Arrow: "right"}, ShapeItem, "@{ shape: text,"},
	}
	for _, tc := range cases {
		node := &Node{ID: "n", Kind: tc.node.Kind, Name: tc.node.Kind, NameSynthesized: true, Arrow: tc.node.Arrow}
		if got := mermaidNodeShape(tc.kind, node, labels, Options{Show: AllShown()}); !strings.HasPrefix(got, tc.mermaid) {
			t.Errorf("%s/%s: written Mermaid shape = %q, want prefix %q", tc.kind, tc.node.Kind, got, tc.mermaid)
		}
		if got := NodeShape(tc.kind, tc.node); got != tc.shape {
			t.Errorf("%s/%s: shape = %q, want %q", tc.kind, tc.node.Kind, got, tc.shape)
		}
	}
}

// Negative cases: no kind is a safe rectangle, and the exported shape set
// holds only names a client can draw.
func TestNodeShapeUnknownAndShapeSet(t *testing.T) {
	if got := NodeShape(KindTree, NodeData{Kind: ""}); got != ShapeRounded {
		t.Errorf("empty kind: shape = %q, want rounded", got)
	}
	if ShapeStadium.Valid() != true || Shape("nonsense").Valid() {
		t.Errorf("Valid: %q and %q", ShapeStadium, Shape("nonsense"))
	}
	if len(Shapes()) != 12 {
		t.Errorf("Shapes() = %v, want 12", Shapes())
	}
}
