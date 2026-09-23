package migrate_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
)

// migrateLayoutFixture migrates testdata/xmi/layout.xmi augmented by the MTIP
// export layout.layout.xml.
func migrateLayoutFixture(t *testing.T) *migrate.Result {
	t.Helper()
	data, err := os.ReadFile("testdata/xmi/layout.xmi")
	if err != nil {
		t.Fatal(err)
	}
	layoutData, err := os.ReadFile("testdata/xmi/layout.layout.xml")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := mtip.Parse(layoutData)
	if err != nil {
		t.Fatalf("mtip.Parse: %v", err)
	}
	r, err := migrate.MigrateOptions("layout.xmi", data, migrate.Options{Layout: layout, LayoutSource: "layout.layout.xml"})
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
	if l.Routes != 4 || l.RoutesWritten != 2 || l.RoutesUnexposed != 1 || l.RoutesDangling != 1 {
		t.Errorf("routes: %+v", l)
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
