package migrate_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
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
		`metadata DiagramLayout::Note about 'Idle accept Start then Running' { text = "On demand."; x = 180; y = 40; width = 100; height = 30; }`,
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
	if s.StylesWritten != 2 || s.Notes != 4 || s.NotesAnchored != 2 {
		t.Errorf("styles written = %d, notes = %d, anchored = %d; want 2, 4 and 2", s.StylesWritten, s.Notes, s.NotesAnchored)
	}
	if s.Dropped["ImageShape"] != 2 {
		t.Errorf("dropped = %v; want 2 ImageShape", s.Dropped)
	}
	wantInOrder(t, "modes entry", reportText(t, r),
		"_diag_modes", "laid out from the diagram's own symbol stream: 2 of 3 shown elements positioned (1 not exposed), 1 of 2 connectors routed (1 no v2 member), 2 of 2 symbols drawn in their own colours or font styled, 3 notes written, 2 anchored, free symbols not represented: 1 ImageShape")
}

// A symbol drawn in its own colours but without usable geometry — a shape the
// stream leaves unsized — still dresses the element the view draws: its Style is
// written and the notes anchored to it stay anchored, only its Layout missing.
func TestStyleSurvivesMissingGeometry(t *testing.T) {
	streams := map[string]string{}
	for k, v := range figureStreams {
		streams[k] = v
	}
	streams["BINARY-modes"] = strings.Replace(streams["BINARY-modes"],
		"<geometry>40, 100, 120, 50</geometry>", "", 1)
	r, err := migrate.Migrate("figures.mdzip", mdzip(t, streams))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantClean(t, "figures.sysml", r)
	notation := string(r.Notation)
	if strings.Contains(notation, "Layout about Plant::Pump::Modes::Idle") {
		t.Errorf("an unsized symbol is positioned:\n%s", notation)
	}
	wantInOrder(t, "dressing without geometry", notation,
		`metadata DiagramLayout::Style about Plant::Pump::Modes::Idle { fill = "#E1E1C3"; line = "#99995C"; text = "#000000"; font = "Arial"; fontSize = 11; bold = true; }`,
		`metadata DiagramLayout::Note about Plant::Pump::Modes::Idle { text = "Moves the coolant."; x = 40; y = 200; width = 150; height = 40; }`)
	s := r.Report.Layout
	if s.StylesWritten != 2 || s.NotesAnchored != 2 || s.NotesFreed != 0 {
		t.Errorf("styles written = %d, notes anchored = %d, freed = %d; want 2, 2 and 0", s.StylesWritten, s.NotesAnchored, s.NotesFreed)
	}
}

// A migrated note anchored on a transition reaches the view's rendering on its
// edge and is drawn in DOT anchored to the transition's route.
func TestMigratedConnectorNoteIsDrawn(t *testing.T) {
	r, err := migrate.Migrate("figures.mdzip", mdzip(t, figureStreams))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	rendering, err := session(t, r).ViewRendering("Plant::Pump::Modes::'Pump Modes'")
	if err != nil {
		t.Fatalf("ViewRendering: %v", err)
	}
	var onEdge []view.Note
	for _, note := range rendering.Notes {
		if note.EdgeFrom != "" {
			onEdge = append(onEdge, note)
		}
	}
	if len(onEdge) != 1 || onEdge[0].Text != "On demand." || onEdge[0].EdgeFrom == onEdge[0].EdgeTo {
		t.Fatalf("notes on edges = %+v; want the transition's alone", onEdge)
	}
	dot, err := rendering.DOTWith(view.Options{Style: view.StyleCameo})
	if err != nil {
		t.Fatalf("DOTWith: %v", err)
	}
	if !strings.Contains(dot, "On demand.") || !strings.Contains(dot, `:on" [shape=point`) {
		t.Errorf("the transition's note is not drawn anchored on its route:\n%s", dot)
	}
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
// stream for every element it places or routes, the stream supplying the rest
// and dressing the view, and lays out every diagram the export does not record;
// the report tells the sources apart.
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
		"metadata DiagramLayout::Layout about Plant::Pump::Modes::Running { x = 260; y = 100; width = 120; height = 50; }",
		"metadata DiagramLayout::Route about 'Idle accept Start then Running' { points = (160, 125, 210, 125, 210, 130, 260, 130); }",
		`metadata DiagramLayout::Style about Plant::Pump::Modes::Idle {`,
		`metadata DiagramLayout::Note about Plant::Pump::Modes::Idle {`)
	if strings.Contains(notation, "about Plant::Pump::Modes::Idle { x = 40;") {
		t.Errorf("the stream's placement of Idle overrides the export's:\n%s", notation)
	}
	wantInOrder(t, "stream fallback", notation,
		"view Partial {",
		"metadata DiagramLayout::Layout about Tank { x = 100; y = 100; width = 120; height = 60; }")
	s := r.Report.Layout
	if s == nil || s.DiagramsJoined != 1 || s.StreamDiagrams != 5 || s.StreamSupplemented != 1 {
		t.Fatalf("layout summary = %+v; want 1 diagram joined and supplemented, 5 from streams", s)
	}
	report := reportText(t, r)
	wantInOrder(t, "sources", report,
		"1 joined views supplemented from their own symbol stream",
		"_diag_partial", "laid out from the diagram's own symbol stream",
		"_diag_modes", "laid out from modes.layout.xml supplemented by the diagram's own symbol stream: 2 of 3 shown elements positioned")
}

// An MTIP export placing and routing everything the Pump Modes stream draws
// leaves the stream nothing to supplement, yet its frame still sizes the canvas.
func TestFrameSurvivesCompleteExport(t *testing.T) {
	place := func(key, id string, top, bottom, left, right int) string {
		return fmt.Sprintf(`<element _dtype="dict" key="%s">
               <relationship_metadata _dtype="dict">
                  <top _dtype="int">%d</top><bottom _dtype="int">%d</bottom>
                  <left _dtype="int">%d</left><right _dtype="int">%d</right>
               </relationship_metadata>
               <id _dtype="str">%s</id><type _dtype="str">sysml.State</type>
            </element>`, key, top, bottom, left, right, id)
	}
	route := func(key, id string, cx, cy, sx, sy int) string {
		return fmt.Sprintf(`<diagramConnector _dtype="dict" key="%s">
               <relationship_metadata _dtype="dict">
                  <clientPoint _dtype="dict"><xCoordinate _dtype="int">%d</xCoordinate><yCoordinate _dtype="int">%d</yCoordinate></clientPoint>
                  <supplierPoint _dtype="dict"><xCoordinate _dtype="int">%d</xCoordinate><yCoordinate _dtype="int">%d</yCoordinate></supplierPoint>
               </relationship_metadata>
               <id _dtype="str">%s</id><type _dtype="str">sysml.Transition</type>
            </diagramConnector>`, key, cx, cy, sx, sy, id)
	}
	export := strings.Replace(modesExport, `<element _dtype="dict" key="0">`,
		place("1", "_st_running", -100, -150, 260, 380)+place("2", "_sm_init", -40, -56, 60, 76)+
			`<element _dtype="dict" key="0">`, 1)
	export = strings.Replace(export, `</element>
      </relationships>`, `</element>
         <diagramConnector _dtype="list">`+route("0", "_t_init", 68, 56, 68, 100)+route("1", "_t_start", 160, 125, 260, 130)+`
         </diagramConnector>
      </relationships>`, 1)
	if export == modesExport {
		t.Fatal("the export fixture did not take the added records")
	}
	layout, err := mtip.Parse([]byte(export))
	if err != nil {
		t.Fatalf("mtip.Parse: %v", err)
	}
	if d := layout.Diagrams[0]; len(d.Placements) != 3 || len(d.Connectors) != 2 || len(d.Malformed) != 0 {
		t.Fatalf("export record = %+v; want 3 placements and 2 connectors", d)
	}
	r, err := migrate.MigrateOptions("figures.mdzip", mdzip(t, figureStreams),
		migrate.Options{Layout: layout, LayoutSource: "modes.layout.xml"})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	wantClean(t, "figures.sysml", r)
	wantInOrder(t, "framed canvas", string(r.Notation),
		"view 'Pump Modes' : StandardViewDefinitions::StateTransitionView {",
		`@DiagramLayout::Canvas { unit = "px"; width = 810; height = 610; }`,
		"metadata DiagramLayout::Layout about Plant::Pump::Modes::Idle { x = 500; y = 300; width = 120; height = 50; }")
	if s := r.Report.Layout; s == nil || s.DiagramsJoined != 1 || s.StreamSupplemented != 0 {
		t.Fatalf("layout summary = %+v; want 1 diagram joined, none supplemented", s)
	}
}
