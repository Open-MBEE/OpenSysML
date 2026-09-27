package view

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// partlyPlaced is an interconnection some Layouts position: two placed parts
// joined by a routed connection, a loose part, a wide definition and a group
// with two members none of which has a place.
func partlyPlaced() *Rendering {
	return &Rendering{
		View:   "V",
		Kind:   KindInterconnection,
		Canvas: &Canvas{Unit: "px", Width: 300, Height: 100, HasSize: true},
		Roots: []*Node{
			{ID: "placed", Kind: "part", Name: "pump", Geometry: &Geometry{X: 10, Y: 10, Width: 100, Height: 40, HasSize: true}},
			{ID: "low", Kind: "part", Name: "tank", Geometry: &Geometry{X: 150, Y: 120, Width: 60, Height: 30, HasSize: true}},
			{ID: "loose", Kind: "part", Name: "spare"},
			{ID: "wide", Kind: "part def", Name: "A rather long definition name"},
			{ID: "group", Kind: "part def", Name: "Group", Children: []*Node{
				{ID: "g1", Kind: "part", Name: "g1"},
				{ID: "g2", Kind: "part", Name: "g2"},
			}},
		},
		Edges: []Edge{
			{From: "placed", To: "low", Kind: EdgeConnection, Route: []Point{{X: 60, Y: 50}, {X: 60, Y: 170}, {X: 150, Y: 170}}},
			{From: "placed", To: "loose", Kind: EdgeConnection},
			{From: "loose", To: "g1", Kind: EdgeFlow},
		},
	}
}

// drawnNodes are the node IDs a written diagram declares, in order: the
// identifiers before `[` in Mermaid, after `as` in PlantUML, before `[` at the
// head of a statement in DOT.
func drawnNodes(t *testing.T, form Form, artifact string) []string {
	t.Helper()
	var pattern *regexp.Regexp
	switch form {
	case FormMermaid:
		pattern = regexp.MustCompile(`(?m)^\s*(?:subgraph )?(\w+) ?\["`)
	case FormPlantUML:
		pattern = regexp.MustCompile(`(?m)^\s*(?:class|rectangle|state|participant) ".*" as (\w+)`)
	case FormDot:
		pattern = regexp.MustCompile(`(?m)^\s*"(\w+)" \[`)
	default:
		t.Fatalf("no node pattern for the %s form", form)
	}
	var ids []string
	for _, match := range pattern.FindAllStringSubmatch(artifact, -1) {
		ids = append(ids, match[1])
	}
	slices.Sort(ids)
	return ids
}

// Positioned holds for a graph-shaped rendering some Layout or Route places,
// and for nothing else: an unplaced graph, a table, or a missing rendering.
func TestRenderingPositioned(t *testing.T) {
	if !partlyPlaced().Positioned() {
		t.Error("a partly placed interconnection is not Positioned")
	}
	loose := partlyPlaced()
	for _, n := range loose.Roots {
		n.Geometry = nil
	}
	for i := range loose.Edges {
		loose.Edges[i].Route = nil
	}
	if loose.Positioned() {
		t.Error("an interconnection without Layouts or Routes is Positioned")
	}
	routed := partlyPlaced()
	for _, n := range routed.Roots {
		n.Geometry = nil
	}
	if !routed.Positioned() {
		t.Error("a Route alone does not make the rendering Positioned")
	}
	table := &Rendering{View: "T", Kind: KindTable, Roots: []*Node{
		{ID: "r", Kind: "part", Name: "r", Geometry: &Geometry{X: 1, Y: 1}},
	}}
	if table.Positioned() {
		t.Error("a table is Positioned")
	}
	var missing *Rendering
	if missing.Positioned() {
		t.Error("a nil rendering is Positioned")
	}
}

// Every graph-shaped form of a partly placed rendering draws the placed nodes
// and the edges between them, and no other, accounting for the rest in its own
// comment syntax; UnplacedStrip draws every node in every form.
func TestGraphFormsDrawOnePlacement(t *testing.T) {
	dot, err := partlyPlaced().DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	placed := drawnNodes(t, FormDot, dot)
	if !slices.Equal(placed, []string{"low", "placed"}) {
		t.Fatalf("DOT draws %v of the partly placed rendering:\n%s", placed, dot)
	}
	notice := "not represented: 5 node(s) without a position, left undrawn, and 2 edge(s) at them\n"
	cases := []struct {
		form    Form
		comment string
		edge    string
	}{
		{FormMermaid, "%% ", "  placed --- low\n"},
		{FormPlantUML, "' ", "placed -[thickness=3]- low\n"},
	}
	for _, tc := range cases {
		artifact, err := partlyPlaced().Write(tc.form)
		if err != nil {
			t.Fatalf("%s: %v", tc.form, err)
		}
		if drawn := drawnNodes(t, tc.form, artifact); !slices.Equal(drawn, placed) {
			t.Errorf("%s draws %v, DOT %v:\n%s", tc.form, drawn, placed, artifact)
		}
		if !strings.Contains(artifact, tc.comment+notice) {
			t.Errorf("%s does not account for the unplaced nodes:\n%s", tc.form, artifact)
		}
		if !strings.Contains(artifact, tc.edge) || strings.Contains(artifact, "loose") || strings.Contains(artifact, "g1") {
			t.Errorf("%s edges are not those between placed nodes:\n%s", tc.form, artifact)
		}
		stripped, err := partlyPlaced().WriteWith(tc.form, Options{Unplaced: UnplacedStrip})
		if err != nil {
			t.Fatalf("%s strip: %v", tc.form, err)
		}
		all := []string{"g1", "g2", "group", "loose", "low", "placed", "wide"}
		if drawn := drawnNodes(t, tc.form, stripped); !slices.Equal(drawn, all) {
			t.Errorf("%s under strip draws %v, not every node:\n%s", tc.form, drawn, stripped)
		}
		if !strings.Contains(stripped, tc.comment+"not represented: 5 node(s) without a position, drawn among the placed ones; the "+string(tc.form)+" form lays every node out itself\n") {
			t.Errorf("%s under strip does not account for the unplaced nodes:\n%s", tc.form, stripped)
		}
		if strings.Count(stripped, " --- ")+strings.Count(stripped, "-[thickness=3]-")+strings.Count(stripped, " -.-> ")+strings.Count(stripped, "-[dashed]->") != 3 {
			t.Errorf("%s under strip does not draw every edge:\n%s", tc.form, stripped)
		}
	}
	checkPlantUMLSyntax(t, written(t, partlyPlaced(), FormPlantUML))
}

// A rendering placing every node, or none, is drawn whole with no accounting.
func TestGraphFormsDrawWhollyPlacedOrUnplaced(t *testing.T) {
	whole := partlyPlaced()
	whole.Roots = whole.Roots[:2]
	whole.Edges = whole.Edges[:1]
	none := partlyPlaced()
	for _, root := range none.Roots {
		root.Geometry = nil
	}
	none.Edges[0].Route = nil
	for _, r := range []*Rendering{whole, none} {
		for _, form := range []Form{FormMermaid, FormPlantUML, FormDot} {
			artifact, err := r.Write(form)
			if err != nil {
				t.Fatalf("%s: %v", form, err)
			}
			if strings.Contains(artifact, "without a position") {
				t.Errorf("%s accounts for unplaced nodes where there is no partial placement:\n%s", form, artifact)
			}
			if want := len(r.Roots) + len(r.Roots[len(r.Roots)-1].Children); len(drawnNodes(t, form, artifact)) != want {
				t.Errorf("%s draws %v, not all %d nodes:\n%s", form, drawnNodes(t, form, artifact), want, artifact)
			}
		}
	}
}

// In a tree, an unplaced node's placed members are drawn detached from the node
// above, as the DOT form draws them; in a clustered kind the node round a
// placed member has a place of its own, so the nesting is kept whole.
func TestOmittedNodeMembersTakeItsPlace(t *testing.T) {
	tree := &Rendering{
		View: "V",
		Kind: KindTree,
		Roots: []*Node{
			{ID: "root", Kind: "package", Name: "Root", Geometry: &Geometry{X: 0, Y: 0, Width: 80, Height: 40, HasSize: true}, Children: []*Node{
				{ID: "mid", Kind: "package", Name: "Mid", Children: []*Node{
					{ID: "leaf", Kind: "part def", Name: "Leaf", Geometry: &Geometry{X: 0, Y: 100, Width: 80, Height: 40, HasSize: true}},
				}},
			}},
		},
	}
	mermaid := tree.Mermaid()
	if !strings.Contains(mermaid, "%% not represented: 1 node(s) without a position, left undrawn\n") {
		t.Errorf("tree Mermaid header:\n%s", mermaid)
	}
	if !slices.Equal(drawnNodes(t, FormMermaid, mermaid), []string{"leaf", "root"}) || strings.Contains(mermaid, " --- ") {
		t.Errorf("tree Mermaid draws the placed nodes detached:\n%s", mermaid)
	}
	dot := written(t, tree, FormDot)
	if !slices.Equal(drawnNodes(t, FormDot, dot), []string{"leaf", "root"}) || strings.Contains(dot, " -> ") {
		t.Errorf("tree DOT draws the placed nodes detached:\n%s", dot)
	}

	clustered := &Rendering{
		View: "V",
		Kind: KindInterconnection,
		Roots: []*Node{
			{ID: "outer", Kind: "part", Name: "outer", Geometry: &Geometry{X: 0, Y: 0, Width: 300, Height: 300, HasSize: true}, Children: []*Node{
				{ID: "inner", Kind: "part", Name: "inner", Children: []*Node{
					{ID: "deep", Kind: "part", Name: "deep", Geometry: &Geometry{X: 20, Y: 20, Width: 80, Height: 40, HasSize: true}},
				}},
			}},
		},
	}
	for _, form := range []Form{FormMermaid, FormPlantUML, FormDot} {
		artifact := written(t, clustered, form)
		if drawn := drawnNodes(t, form, artifact); !slices.Equal(drawn, []string{"deep", "inner", "outer"}) {
			t.Errorf("%s draws %v of a cluster placed by its member:\n%s", form, drawn, artifact)
		}
		if strings.Contains(artifact, "without a position") {
			t.Errorf("%s accounts for an unplaced node where a cluster is placed by its member:\n%s", form, artifact)
		}
	}
}

// A placement asked for by a name none has is refused by every form that
// returns an error, as the DOT form refuses it.
func TestPlantUMLRefusesUnknownUnplaced(t *testing.T) {
	_, err := partlyPlaced().PlantUMLWith(Options{Unplaced: "hide"})
	var unknown *UnknownUnplacedError
	if !errors.As(err, &unknown) || unknown.Name != "hide" {
		t.Fatalf("PlantUML with an unknown placement: %v", err)
	}
}

// written is r in form, failing the test when the form refuses it.
func written(t *testing.T, r *Rendering, form Form) string {
	t.Helper()
	artifact, err := r.Write(form)
	if err != nil {
		t.Fatalf("%s: %v", form, err)
	}
	return artifact
}
