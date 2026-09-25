package migrate_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
)

// An MTIP export placing the Pump Modes diagram's states elsewhere than its stream does.
const modesExport = `<?xml version="1.0" encoding="UTF-8"?><packet>
   <metadata><mtipVersion>2022x v1.0.0</mtipVersion></metadata>
   <data>
      <attributes _dtype="dict">
         <attribute _dtype="dict" key="name"><attribute _dtype="str" key="value">Pump Modes</attribute></attribute>
      </attributes>
      <id _dtype="dict"><cameo _dtype="str">_diag_modes</cameo></id>
      <relationships _dtype="dict">
         <element _dtype="list">
            <element _dtype="dict" key="0">
               <relationship_metadata _dtype="dict">
                  <top _dtype="int">-300</top>
                  <bottom _dtype="int">-350</bottom>
                  <left _dtype="int">500</left>
                  <right _dtype="int">620</right>
               </relationship_metadata>
               <id _dtype="str">_st_idle</id>
               <type _dtype="str">sysml.State</type>
            </element>
         </element>
      </relationships>
      <type _dtype="str">sysml.Diagram</type>
   </data>
</packet>`

// A diagram's own symbol stream lays out its view: the frame sizes the canvas, each
// state's symbol is a Layout, a transition's path a Route, a symbol's own colours and
// font a Style, a comment's symbol a Note anchored to the state its anchor reaches, a
// text box a free Note, and a pasted image is counted as not represented rather than
// embedded. The initial pseudostate and its transition, which no member names, are
// accounted for rather than positioned.
func TestStreamLaysOutView(t *testing.T) {
	r, err := migrate.Migrate("figures.mdzip", mdzip(t, figureStreams))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantClean(t, "figures.sysml", r)
	wantInOrder(t, "stream metadata", string(r.Notation),
		"view 'Pump Modes' : StandardViewDefinitions::StateTransitionView {",
		`@DiagramLayout::Canvas { unit = "px"; width = 810; height = 610; }`,
		"metadata DiagramLayout::Layout about Plant::Pump::Modes::Idle { x = 40; y = 100; width = 120; height = 50; }",
		"metadata DiagramLayout::Layout about Plant::Pump::Modes::Running { x = 260; y = 100; width = 120; height = 50; }",
		"metadata DiagramLayout::Route about 'Idle accept Start then Running' { points = (160, 125, 210, 125, 210, 130, 260, 130); }",
		`metadata DiagramLayout::Style about Plant::Pump::Modes::Idle { fill = "#E1E1C3"; line = "#99995C"; text = "#000000"; font = "Arial"; fontSize = 11; bold = true; }`,
		`metadata DiagramLayout::Note about Plant::Pump::Modes::Idle { text = "Moves the coolant."; x = 40; y = 200; width = 150; height = 40; }`,
		`@DiagramLayout::Note { text = "Draft only"; x = 600; y = 20; width = 80; height = 12; }`,
		"render Views::asInterconnectionDiagram;")
	if strings.Contains(string(r.Notation), "logo.png") {
		t.Errorf("the pasted image is embedded:\n%s", r.Notation)
	}
	s := r.Report.Layout
	if s == nil {
		t.Fatal("no layout summary for a stream-laid-out migration")
	}
	if s.Source != "the diagrams' own symbol streams" {
		t.Errorf("source = %q", s.Source)
	}
	if s.StreamDiagrams != 6 || s.DiagramsJoined != 0 {
		t.Errorf("stream diagrams = %d, joined = %d; want 6 and 0", s.StreamDiagrams, s.DiagramsJoined)
	}
	if s.StylesWritten != 2 || s.Notes != 3 || s.NotesAnchored != 1 {
		t.Errorf("styles written = %d, notes = %d, anchored = %d; want 2, 3 and 1", s.StylesWritten, s.Notes, s.NotesAnchored)
	}
	if s.Dropped["ImageShape"] != 2 {
		t.Errorf("dropped = %v; want 2 ImageShape", s.Dropped)
	}
	wantInOrder(t, "modes entry", reportText(t, r),
		"_diag_modes", "laid out from the diagram's own symbol stream: 2 of 3 shown elements positioned (1 not exposed), 1 of 2 connectors routed (1 no v2 member), 2 of 2 symbols drawn in their own colours or font styled, 2 notes written, 1 anchored, free symbols not represented: 1 ImageShape")
}

// reportText renders r's report as text.
func reportText(t *testing.T, r *migrate.Result) string {
	t.Helper()
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	return report.String()
}

// An MTIP export's record of a diagram takes precedence over the diagram's own
// stream for geometry, while the stream still dresses the view and lays out every
// diagram the export does not record; the report tells the two sources apart.
func TestExportPrecedesStream(t *testing.T) {
	layout, err := mtip.Parse([]byte(modesExport))
	if err != nil {
		t.Fatalf("mtip.Parse: %v", err)
	}
	r, err := migrate.MigrateOptions("figures.mdzip", mdzip(t, figureStreams),
		migrate.Options{Layout: layout, LayoutSource: "modes.layout.xml"})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	wantClean(t, "figures.sysml", r)
	notation := string(r.Notation)
	wantInOrder(t, "export geometry", notation,
		"view 'Pump Modes' : StandardViewDefinitions::StateTransitionView {",
		"metadata DiagramLayout::Layout about Plant::Pump::Modes::Idle { x = 500; y = 300; width = 120; height = 50; }",
		`metadata DiagramLayout::Style about Plant::Pump::Modes::Idle {`,
		`metadata DiagramLayout::Note about Plant::Pump::Modes::Idle {`)
	if strings.Contains(notation, "about Plant::Pump::Modes::Running {") || strings.Contains(notation, "Route about 'Idle accept Start then Running'") {
		t.Errorf("the stream's geometry leaks into a diagram the export records:\n%s", notation)
	}
	wantInOrder(t, "stream fallback", notation,
		"view Partial {",
		"metadata DiagramLayout::Layout about Tank { x = 100; y = 100; width = 120; height = 60; }")
	s := r.Report.Layout
	if s == nil || s.DiagramsJoined != 1 || s.StreamDiagrams != 5 {
		t.Fatalf("layout summary = %+v; want 1 diagram joined and 5 from streams", s)
	}
	report := reportText(t, r)
	wantInOrder(t, "sources", report,
		"_diag_partial", "laid out from the diagram's own symbol stream",
		"_diag_modes", "laid out from modes.layout.xml: 1 of 1 shown elements positioned")
}
