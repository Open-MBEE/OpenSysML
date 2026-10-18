package migrate_test

import (
	"bytes"
	"strings"
	"testing"
)

// A tree view exposes only what a symbol of its own stands for. The attributes,
// constraint property and enumeration literals a class box lists as compartment
// rows, and the part and attribute whose only symbols are role labels on the
// association lines, are drawn by the exposed definitions and lines; exposed by
// themselves a tool would draw each as a node. The port keeps its border symbol
// and its Layout, and the line backing an attribute's typing is routed onto the
// attribute's edge, run from the owner's box whichever end the tool drew first;
// a private attribute, which the view cannot name, takes no route.
func TestGoldenCompartmentRowsAreNotExposed(t *testing.T) {
	r := migrateStream(t, "compartment_rows")
	l := r.Report.Layout
	if l == nil || l.StreamDiagrams != 1 || l.PlacementsWritten != 4 || l.PlacementsOnEdges != 7 || l.PlacementsUnexposed != 0 || l.Routes != 3 || l.RoutesWritten != 2 || l.RoutesUnexposed != 1 {
		t.Fatalf("layout summary: %+v", l)
	}
	checkGolden(t, "testdata/xmi/compartment_rows.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/compartment_rows.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "compartment_rows.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	notation := string(r.Notation)
	for _, want := range []string{
		"expose Vehicle;",
		"expose Engine;",
		"expose Mode;",
		"expose Blocks::Vehicle::fuel;",
		"expose EngineToVehicle;",
		"metadata DiagramLayout::Layout about Vehicle { x = 40; y = 20; width = 160; height = 120; }",
		"metadata DiagramLayout::Layout about Blocks::Vehicle::fuel { x = 195; y = 60; width = 10; height = 10; }",
		"metadata DiagramLayout::Layout about Engine { x = 40; y = 220; width = 120; height = 50; }",
		"metadata DiagramLayout::Layout about Mode { x = 300; y = 220; width = 120; height = 60; }",
		"metadata DiagramLayout::Route about EngineToVehicle { points = (100, 220, 100, 140); }",
		"metadata DiagramLayout::Route about Blocks::Vehicle::mode { points = (200, 100, 360, 220); }",
		"private attribute trim : Mode;",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	if strings.Contains(notation, "Route about Blocks::Vehicle::trim") {
		t.Errorf("output routes the line of the private attribute trim, which the view cannot name")
	}
	for _, row := range []string{"speed", "mass", "limit", "engine", "mode", "trim", "eco", "sport"} {
		if strings.Contains(notation, "expose Blocks::Vehicle::"+row+";") || strings.Contains(notation, "expose Blocks::Mode::"+row+";") || strings.Contains(notation, "expose "+row+";") {
			t.Errorf("output exposes compartment row or end label %s", row)
		}
	}
	if strings.Count(notation, "expose ") != 5 {
		t.Errorf("want 5 exposes, got %d", strings.Count(notation, "expose "))
	}
}
