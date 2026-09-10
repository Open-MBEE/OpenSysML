package view

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// fixtureText is the source of a fixture, for checking what an origin spans.
func fixtureText(t *testing.T, file string) *source.SourceFile {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return source.New(file, content)
}

// flatten is every node of a rendering, keyed by ID.
func flatten(rendering *Rendering) map[string]NodeData {
	out := map[string]NodeData{}
	for _, node := range rendering.Data().Nodes {
		out[node.ID] = node
	}
	return out
}

// nodeNamed is the first node of a rendering with the given name.
func nodeNamed(t *testing.T, rendering *Rendering, name string) NodeData {
	t.Helper()
	for _, node := range rendering.Data().Nodes {
		if node.Name == name {
			return node
		}
	}
	t.Fatalf("rendering has no node named %q", name)
	return NodeData{}
}

// A tree node's origin spans the declaration it was built from.
func TestTreeNodeOriginsSpanTheirDeclaration(t *testing.T) {
	rendering := render(t, "tree.sysml", "VehicleViews::vehicleView")
	sf := fixtureText(t, "tree.sysml")
	for name, want := range map[string]string{
		"Vehicles::Vehicle": "part def Vehicle",
		"engine":            "part engine : Engine",
		"wheels":            "part wheels[4] : Wheel",
		"Vehicles::Engine":  "part def Engine",
		"cylinder":          "part cylinder[4] : Cylinder",
	} {
		node := nodeNamed(t, rendering, name)
		if node.Origin.Doc != "tree.sysml" {
			t.Errorf("node %s: origin document = %q, want tree.sysml", name, node.Origin.Doc)
			continue
		}
		if text := sf.Text(node.Origin.Span); !strings.HasPrefix(text, want) {
			t.Errorf("node %s: origin spans %q, want it to start with %q", name, text, want)
		}
	}
}

// A node's origin names the declared identifier within the declaration it spans,
// so a reader is taken to the name rather than the whole element.
func TestNodeOriginsNameTheDeclaredIdentifier(t *testing.T) {
	rendering := render(t, "tree.sysml", "VehicleViews::vehicleView")
	sf := fixtureText(t, "tree.sysml")
	for name, want := range map[string]string{
		"Vehicles::Vehicle": "Vehicle",
		"engine":            "engine",
		"wheels":            "wheels",
	} {
		node := nodeNamed(t, rendering, name)
		if node.Origin.Name.Len == 0 {
			t.Errorf("node %s: origin names no identifier", name)
			continue
		}
		if text := sf.Text(node.Origin.Name); text != want {
			t.Errorf("node %s: origin names %q, want %q", name, text, want)
		}
		if node.Origin.Name.Offset < node.Origin.Span.Offset ||
			node.Origin.Name.Offset+node.Origin.Name.Len > node.Origin.Span.Offset+node.Origin.Span.Len {
			t.Errorf("node %s: named span %v is outside the declaration %v", name, node.Origin.Name, node.Origin.Span)
		}
	}
}

// An interconnection rendering locates its features and the connector each edge
// was declared as.
func TestInterconnectionOriginsLocateFeaturesAndConnectors(t *testing.T) {
	rendering := render(t, "interconnection.sysml", "PlantViews::loopView")
	sf := fixtureText(t, "interconnection.sysml")
	if len(rendering.Edges) == 0 {
		t.Fatal("interconnection rendering has no edges")
	}
	nodes := flatten(rendering)
	for _, edge := range rendering.Edges {
		if !edge.Origin.Located() {
			t.Errorf("edge %s→%s carries no origin", edge.From, edge.To)
			continue
		}
		text := sf.Text(edge.Origin.Span)
		if !strings.HasPrefix(text, "connect") && !strings.HasPrefix(text, "interface") && !strings.HasPrefix(text, "flow") {
			t.Errorf("edge %s→%s origin spans %q, want the connector's declaration", edge.From, edge.To, text)
		}
		for _, id := range []string{edge.From, edge.To} {
			if !nodes[id].Origin.Located() {
				t.Errorf("node %s (%s) carries no origin", id, nodes[id].Name)
			}
		}
	}
}

// A state rendering locates each state and the transition each edge was
// declared as, both of which come from the lowered graph rather than a symbol;
// an entry edge is located at the entry transition written after `entry;`.
func TestStateOriginsComeFromTheLoweredGraph(t *testing.T) {
	rendering := render(t, "state.sysml", "MachineViews::vehicleStates")
	sf := fixtureText(t, "state.sysml")
	for _, name := range []string{"off", "operating", "idle", "moving"} {
		node := nodeNamed(t, rendering, name)
		if text := sf.Text(node.Origin.Span); !strings.HasPrefix(text, "state "+name) {
			t.Errorf("state %s: origin spans %q, want the state's declaration", name, text)
		}
	}
	var transitions, entries int
	for _, edge := range rendering.Edges {
		if !edge.Origin.Located() {
			t.Errorf("edge %s→%s carries no origin", edge.From, edge.To)
			continue
		}
		text := sf.Text(edge.Origin.Span)
		switch {
		case strings.HasPrefix(text, "transition"), strings.HasPrefix(text, "accept"):
			transitions++
		case strings.HasPrefix(text, "then "), strings.HasPrefix(text, "if "):
			entries++
		default:
			t.Errorf("transition edge origin spans %q, want the transition's declaration", text)
		}
	}
	if transitions == 0 || entries == 0 {
		t.Errorf("%d transition and %d entry edges carry an origin, want some of each", transitions, entries)
	}
}

// An action rendering locates its nodes and the successions between them.
func TestActionOriginsComeFromTheLoweredGraph(t *testing.T) {
	rendering := render(t, "action.sysml", "FlowViews::driveView")
	sf := fixtureText(t, "action.sysml")
	for _, node := range rendering.Data().Nodes {
		if node.Origin.Located() && sf.Text(node.Origin.Span) == "" {
			t.Errorf("node %s: origin %v spans nothing", node.Name, node.Origin)
		}
	}
	located := 0
	for _, edge := range rendering.Edges {
		if edge.Origin.Located() {
			located++
		}
	}
	if located == 0 {
		t.Error("no succession or flow edge carries an origin")
	}
}

// A table rendering locates the element each row reports, one origin per row.
func TestTableRowOriginsMatchTheirRows(t *testing.T) {
	rendering := render(t, "table.sysml", "TableViews::partsTable")
	if len(rendering.Rows) != len(rendering.RowOrigins) {
		t.Fatalf("%d rows but %d origins", len(rendering.Rows), len(rendering.RowOrigins))
	}
	sf := fixtureText(t, "table.sysml")
	for i, row := range rendering.Data().Rows {
		if !row.Origin.Located() {
			continue
		}
		name := strings.TrimPrefix(row.Cells[0], "'")
		if simple := name[strings.LastIndex(name, ":")+1:]; !strings.Contains(sf.Text(row.Origin.Span), strings.TrimSuffix(simple, "'")) {
			t.Errorf("row %d (%s): origin spans %q, which does not declare it", i, row.Cells[0], sf.Text(row.Origin.Span))
		}
	}
}

// A rendering of exposed elements alone — what a document with no view declared
// is rendered as — goes through the same renderer and names no view.
func TestRenderExposedRendersWithoutAView(t *testing.T) {
	r, idx := loadFixture(t, "tree.sysml")
	rendering, err := r.RenderExposed(
		[]*symbols.Symbol{lookup(t, idx, "Vehicles::Vehicle")}, KindTree, "no view declared; rendering Vehicles::Vehicle directly")
	if err != nil {
		t.Fatalf("render exposed: %v", err)
	}
	if rendering.View != "" {
		t.Errorf("View = %q, want empty: no view was declared", rendering.View)
	}
	if rendering.Stated != "no view declared; rendering Vehicles::Vehicle directly" {
		t.Errorf("Stated = %q", rendering.Stated)
	}
	node := nodeNamed(t, rendering, "Vehicles::Vehicle")
	if !node.Origin.Located() {
		t.Error("the exposed element's node carries no origin")
	}
}

// An unsupported kind asked for directly is refused rather than rendered as
// another kind.
func TestRenderExposedRefusesAnUnsupportedKind(t *testing.T) {
	r, idx := loadFixture(t, "tree.sysml")
	_, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, "Vehicles::Vehicle")}, KindGeometry, "")
	var unsupported *UnsupportedKindError
	if !errors.As(err, &unsupported) {
		t.Fatalf("err = %v, want an *UnsupportedKindError", err)
	}
	if unsupported.Kind != KindGeometry {
		t.Errorf("Kind = %q, want %q", unsupported.Kind, KindGeometry)
	}
}
