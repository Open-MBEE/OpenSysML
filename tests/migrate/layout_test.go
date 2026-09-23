package migrate_test

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
)

// migrateLayoutFixture migrates testdata/xmi/layout.xmi augmented by the MTIP
// export layout.layout.xml.
func migrateLayoutFixture(t *testing.T) *migrate.Result {
	t.Helper()
	return migrateLaidOut(t, "layout")
}

// migrateLaidOut migrates testdata/xmi/<name>.xmi augmented by the MTIP export
// <name>.layout.xml beside it.
func migrateLaidOut(t *testing.T, name string) *migrate.Result {
	t.Helper()
	data, err := os.ReadFile("testdata/xmi/" + name + ".xmi")
	if err != nil {
		t.Fatal(err)
	}
	layoutData, err := os.ReadFile("testdata/xmi/" + name + ".layout.xml")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := mtip.Parse(layoutData)
	if err != nil {
		t.Fatalf("mtip.Parse: %v", err)
	}
	r, err := migrate.MigrateOptions(name+".xmi", data, migrate.Options{Layout: layout, LayoutSource: name + ".layout.xml"})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	return r
}

// The notation and report of the augmented migration are pinned, and the
// notation analyses clean — the DiagramLayout annotations must type-check.
func TestGoldenLayout(t *testing.T) {
	r := migrateLayoutFixture(t)
	checkGolden(t, "testdata/xmi/layout.layout.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/layout.layout.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "layout.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// Every edge kind a diagram draws is routed once its member is named and exposed, every
// connector record not routed is accounted for by kind and reason, and the notation analyses clean.
func TestGoldenEdgeLayout(t *testing.T) {
	r := migrateLaidOut(t, "diagram_edges")
	checkGolden(t, "testdata/xmi/diagram_edges.layout.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/diagram_edges.layout.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "diagram_edges.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	l := r.Report.Layout
	if l == nil {
		t.Fatal("no layout summary")
	}
	if l.Routes != 20 || l.RoutesWritten != 10 || l.RoutesUnexposed != 9 || l.RoutesDangling != 1 {
		t.Errorf("routes: %+v", l)
	}
	// An edge of the graph the activity's rendering does not draw is exposed, not swallowed.
	if !strings.Contains(string(r.Notation), "expose 'set speed.value = target';") {
		t.Errorf("the activity view does not expose the binding its rendering does not draw:\n%s", r.Notation)
	}
	// A flow several edges carry is named for the shown one, though an unshown one is written first.
	if !strings.Contains(string(r.Notation), "Route about 'gain.result to set speed.value'") {
		t.Errorf("the shown twin of a flow written once is not routed:\n%s", r.Notation)
	}
	// A transition written once per trigger routes every transition it was written as.
	if !strings.Contains(string(r.Notation), "Route about 'Halted accept Go then Running', 'Halted accept Resume then Running'") {
		t.Errorf("the two-trigger transition is not routed as both of its transitions:\n%s", r.Notation)
	}
	// A placement of a transition, which the state rendering draws as an edge, positions no node.
	if l.PlacementsUnexposed != 4 || strings.Contains(string(r.Notation), "Layout about halt") {
		t.Errorf("placements: %+v", l)
	}
	wantKinds := []migrate.RouteKind{
		{Kind: "BindingConnector", Reason: "written", Count: 1},
		{Kind: "Connector", Reason: "unnamed", Count: 1},
		{Kind: "Connector", Reason: "written", Count: 3},
		{Kind: "ControlFlow", Reason: "unnamed", Count: 1},
		{Kind: "ControlFlow", Reason: "written", Count: 2},
		{Kind: "Dependency", Reason: "not drawn", Count: 1},
		{Kind: "Generalization", Reason: "no v2 member", Count: 1},
		{Kind: "Include", Reason: "not drawn", Count: 1},
		{Kind: "ObjectFlow", Reason: "not drawn", Count: 1},
		{Kind: "ObjectFlow", Reason: "written", Count: 1},
		{Kind: "Satisfy", Reason: "not drawn", Count: 1},
		{Kind: "Transition", Reason: "dangling", Count: 1},
		{Kind: "Transition", Reason: "no v2 member", Count: 1},
		{Kind: "Transition", Reason: "written", Count: 3},
		{Kind: "Verify", Reason: "not drawn", Count: 1},
	}
	if !reflect.DeepEqual(l.RoutesByKind, wantKinds) {
		t.Errorf("routes by kind:\n got %+v\nwant %+v", l.RoutesByKind, wantKinds)
	}
	// The view text is the same with or without the export: naming never reads it.
	plain := migrateFixtureFile(t, "diagram_edges")
	var stripped []string
	for _, line := range strings.Split(string(r.Notation), "\n") {
		if !strings.Contains(line, "DiagramLayout::") {
			stripped = append(stripped, line)
		}
	}
	if got := strings.Join(stripped, "\n"); got != string(plain.Notation) {
		t.Errorf("the laid-out notation differs from the plain one beyond its DiagramLayout annotations:\n%s", got)
	}
}

// The routes a migrated view carries reach the DOT form as pinned edge splines for each
// edge kind, labelled by name only where the edge has no text of its own.
func TestMigratedRoutesRenderPinned(t *testing.T) {
	r := migrateLaidOut(t, "diagram_edges")
	s := session(t, r)
	for view, wants := range map[string][]string{
		"Structure::Vehicle::Drive::Driving": {
			`"n5" -> "n1" [label="'start to gain'", pos="60,160 60,160 60,190 60,190"];`,
			`"n3" -> "n4" [label="finish", pos="60,0 60,0 60,40 60,40"];`,
			`[label="result to value", style=dashed, pos="110,60 110,60 140,60 140,60 140,60 140,140 140,140 140,140 110,140 110,140"];`,
		},
		"Behavior::Modes::Modes": {
			`"n1" -> "n2" [label="accept Go", pos="200,110 200,110 110,110 110,110"];`,
			`"n2" -> "n3" [label="accept Stop", pos="250,40 250,40 250,90 250,90"];`,
			`"n3" -> "n2" [label="accept Go", pos="280,90 280,90 320,90 320,90 320,90 320,40 320,40 320,40 280,40 280,40"];`,
			`"n3" -> "n2" [label="accept Resume", pos="280,90 280,90 320,90 320,90 320,90 320,40 320,40 320,40 280,40 280,40"];`,
		},
		"Structure::Vehicle::'Vehicle Internals'": {
			`[label="'engine to wheel'", arrowhead=none, penwidth=3, pos="200,100 200,100 120,100 120,100"];`,
			`[label="'engine to wheel 2'", arrowhead=none, penwidth=3, pos="200,80 200,80 160,60 160,60 160,60 120,80 120,80"];`,
			`[label="drive", arrowhead=none, penwidth=3, pos="200,90 200,90 120,90 120,90"];`,
			`[label="'mass = limit'", arrowhead=none, pos="200,10 200,10 120,10 120,10"];`,
		},
	} {
		rendering, err := s.ViewRendering(view)
		if err != nil {
			t.Fatalf("render %s: %v", view, err)
		}
		dot, err := rendering.DOT()
		if err != nil {
			t.Fatalf("DOT of %s: %v", view, err)
		}
		for _, want := range wants {
			if !strings.Contains(dot, want) {
				t.Errorf("DOT of %s lacks %q:\n%s", view, want, dot)
			}
		}
	}
}

// Zero options must migrate exactly as Migrate does: notation and text report
// byte-identical for every fixture.
func TestZeroOptionsMatchMigrate(t *testing.T) {
	for _, name := range append(append([]string{"vehicle"}, constructFixtures...), "layout") {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/xmi/" + name + ".xmi")
			if err != nil {
				t.Fatal(err)
			}
			plain, err := migrate.Migrate(name+".xmi", data)
			if err != nil {
				t.Fatalf("Migrate: %v", err)
			}
			opts, err := migrate.MigrateOptions(name+".xmi", data, migrate.Options{})
			if err != nil {
				t.Fatalf("MigrateOptions: %v", err)
			}
			if !bytes.Equal(plain.Notation, opts.Notation) {
				t.Error("notation differs")
			}
			var a, b bytes.Buffer
			if err := plain.Report.WriteText(&a); err != nil {
				t.Fatal(err)
			}
			if err := opts.Report.WriteText(&b); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(a.Bytes(), b.Bytes()) {
				t.Error("text report differs")
			}
		})
	}
}

// A layout export whose diagram records match no diagram of the model was
// exported from a different project, and is an error rather than a report.
func TestLayoutProjectMismatch(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/layout.xmi")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := mtip.Parse([]byte(`<packet><metadata><mtipVersion>2022x</mtipVersion></metadata><data><id _dtype="dict"><cameo _dtype="str">_other</cameo></id><relationships _dtype="dict"><element _dtype="list"/></relationships><type _dtype="str">sysml.BlockDefinitionDiagram</type></data></packet>`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = migrate.MigrateOptions("layout.xmi", data, migrate.Options{Layout: foreign, LayoutSource: "foreign.xml"})
	if err == nil {
		t.Fatal("expected the project mismatch error")
	}
	want := "foreign.xml: none of its 1 diagram records matches a diagram of layout.xmi; the layout file was exported from a different project"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
}

// The layout summary accounts for every diagram record and every shown element.
func TestLayoutSummary(t *testing.T) {
	r := migrateLayoutFixture(t)
	l := r.Report.Layout
	if l == nil {
		t.Fatal("no layout summary")
	}
	if l.Diagrams != 3 || l.DiagramsJoined != 2 || l.DiagramsUnmatched != 1 {
		t.Errorf("diagrams: %+v", l)
	}
	if l.Placements != 7 || l.PlacementsWritten != 5 || l.PlacementsUnexposed != 1 || l.PlacementsDangling != 1 {
		t.Errorf("placements: %+v", l)
	}
	if l.Routes != 4 || l.RoutesWritten != 1 || l.RoutesUnexposed != 2 || l.RoutesDangling != 1 {
		t.Errorf("routes: %+v", l)
	}
	// The tree diagram draws no connection, so its connector's route is not
	// pinned; the control flow the interconnection diagram shows is not either.
	wantKinds := []migrate.RouteKind{
		{Kind: "Connector", Reason: "not drawn", Count: 1},
		{Kind: "Connector", Reason: "written", Count: 1},
		{Kind: "ControlFlow", Reason: "not drawn", Count: 1},
		{Kind: "Dependency", Reason: "dangling", Count: 1},
	}
	if !reflect.DeepEqual(l.RoutesByKind, wantKinds) {
		t.Errorf("routes by kind: %+v, want %+v", l.RoutesByKind, wantKinds)
	}
	if l.Malformed != 1 || l.Unsupported["fillColor"] != 1 {
		t.Errorf("malformed/unsupported: %+v", l)
	}
	if !strings.Contains(r.Report.Summary(), "laid out 2 of 2 diagrams from layout.layout.xml") {
		t.Errorf("summary: %s", r.Report.Summary())
	}
	var unmapped int
	for _, e := range r.Report.Entries {
		if e.Kind == "Layout" && e.Verdict == migrate.Unmapped {
			unmapped++
		}
	}
	if unmapped != 2 {
		t.Errorf("layout entries: %d, want one per unmatched record and one per malformed record", unmapped)
	}
}

// A record joining a diagram the migration does not write as a view — its host
// is written without a body — lays out nothing, and the summary counts it as
// unmatched rather than joined.
func TestLayoutDiagramWithoutWrittenView(t *testing.T) {
	xmi := `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:diagram="http://www.example.com/tool/diagram">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
    <packagedElement xmi:type="uml:Package" xmi:id="_pkg" name="P">
      <packagedElement xmi:type="uml:Class" xmi:id="_blk_wheel" name="Wheel"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_blk_vehicle" name="Vehicle">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_prop_w" name="wheel" type="_blk_wheel"/>
        <ownedConnector xmi:type="uml:Connector" xmi:id="_conn" name="drive">
          <end xmi:type="uml:ConnectorEnd" xmi:id="_ce_1" role="_prop_w"/>
          <end xmi:type="uml:ConnectorEnd" xmi:id="_ce_2" role="_prop_w"/>
        </ownedConnector>
      </packagedElement>
    </packagedElement>
    <xmi:Extension extender="Example UML Tool 1.0">
      <modelExtension>
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_diag_unhosted" name="Drive Internals" ownerOfDiagram="_conn">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject type="SysML Internal Block Diagram" umlType="Composite Structure Diagram">
                <diagramContents><usedElements>_blk_wheel</usedElements></diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>
      </modelExtension>
    </xmi:Extension>
  </uml:Model>
  <sysml:Block xmi:id="_s_wheel2" base_Class="_blk_wheel"/>
  <sysml:Block xmi:id="_s_wheel" base_Class="_blk_wheel"/>
</xmi:XMI>`
	layout := &mtip.Export{Diagrams: []mtip.Diagram{{
		ID:          "_diag_unhosted",
		Name:        "Drive Internals",
		Placements:  []mtip.Placement{{ID: "_blk_wheel", X: 10, Y: 20, Width: 100, Height: 40}},
		Unsupported: map[string]int{},
	}}}
	r, err := migrate.MigrateOptions("unhosted.xmi", []byte(xmi), migrate.Options{Layout: layout, LayoutSource: "unhosted.xml"})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	l := r.Report.Layout
	if l == nil {
		t.Fatal("no layout summary")
	}
	if l.Diagrams != 1 || l.DiagramsJoined != 0 || l.DiagramsUnmatched != 1 {
		t.Errorf("diagrams: %+v", l)
	}
	var row *migrate.Entry
	for i := range r.Report.Entries {
		e := &r.Report.Entries[i]
		if e.Kind == "Layout" {
			row = e
		}
	}
	if row == nil || row.ID != "_diag_unhosted" || row.Note != "matches a diagram the migration does not write as a view" {
		t.Errorf("layout row: %+v", row)
	}
	if strings.Contains(string(r.Notation), "DiagramLayout::") {
		t.Errorf("notation lays out a view never written:\n%s", r.Notation)
	}
}
