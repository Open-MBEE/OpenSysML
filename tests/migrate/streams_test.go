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
	if s.StreamDiagrams != 7 || s.DiagramsJoined != 0 {
		t.Errorf("stream diagrams = %d, joined = %d; want 7 and 0", s.StreamDiagrams, s.DiagramsJoined)
	}
	if s.StylesWritten != 2 || s.Notes != 4 || s.NotesAnchored != 2 {
		t.Errorf("styles written = %d, notes = %d, anchored = %d; want 2, 4 and 2", s.StylesWritten, s.Notes, s.NotesAnchored)
	}
	if s.Pictures != 6 || s.PicturesWritten != 3 || s.PicturesUndrawn != 1 {
		t.Errorf("pictures = %d, written = %d, undrawn = %d; want 6, 3 and 1", s.Pictures, s.PicturesWritten, s.PicturesUndrawn)
	}
	if s.Dropped["ImageShape"] != 2 {
		t.Errorf("dropped = %v; want the 2 ImageShape without bytes to write", s.Dropped)
	}
	report := reportText(t, r)
	wantInOrder(t, "modes entry", report,
		"_diag_modes", "laid out from the diagram's own symbol stream: 2 of 3 shown elements positioned (1 not exposed), 1 of 2 connectors routed (1 no v2 member), 2 pasted images written as images/Plant_from_the_north.png, 2 pasted images not written: the pasted image \"logo.png\" is not in the archive and the pasted image of symbol _sym_torn has bytes that do not read (octet 19 is \"xx\", not a hexadecimal byte), 2 of 2 symbols drawn in their own colours or font styled, 3 notes written, 2 anchored, free symbols not represented: 2 ImageShape")
	wantInOrder(t, "roster entry", report,
		"_diag_roster", "1 pasted image written as images/Plant_from_the_north.png, which a view rendered asElementTable does not draw")
	wantInOrder(t, "layout summary", report,
		"# pasted images: 3 of 6 written as files and drawn by the view, 1 written on a view whose table rendering does not draw them")
	wantNote(t, r, "_sym_torn", migrate.Unmapped,
		"the pasted image's bytes do not read: octet 19 is \"xx\", not a hexadecimal byte")
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

// A picture pasted over one element symbol and under another drawn after it is written
// under both, so no symbol is hidden, and the diagram's note says which loss that is.
func TestPictureBetweenSymbolsIsDrawnUnder(t *testing.T) {
	streams := map[string]string{}
	for k, v := range figureStreams {
		streams[k] = v
	}
	modes := streams["BINARY-modes"]
	running := modes[strings.Index(modes, `<mdElement elementClass="State" xmi:id="_sym_running">`):]
	running = running[:strings.Index(running, `<mdElement elementClass="Transition"`)]
	modes = strings.Replace(modes, running, "", 1)
	modes = strings.Replace(modes, "<geometry>350, 120, 40, 40</geometry>", "<geometry>100, 120, 250, 40</geometry>", 1)
	streams["BINARY-modes"] = strings.Replace(modes, "</mdOwnedViews>", running+"\n</mdOwnedViews>", 1)
	r, err := migrate.Migrate("figures.mdzip", mdzip(t, streams))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantClean(t, "figures.sysml", r)
	notation := string(r.Notation)
	sticker := `@DiagramLayout::Picture { location = "images/Plant_from_the_north.png"; x = 100; y = 120; width = 250; height = 40; }`
	if !strings.Contains(notation, sticker) || strings.Contains(notation, "height = 40; above = true;") {
		t.Errorf("the picture between Idle and Running is not drawn under both:\n%s", notation)
	}
	wantInOrder(t, "modes entry", reportText(t, r),
		"_diag_modes", "2 pasted images written as images/Plant_from_the_north.png, 1 pasted image drawn under the 1 element symbol it lay over, since symbols drawn after it lie over it")
	wantInOrder(t, "layout summary", reportText(t, r),
		"# pasted images: 3 of 6 written as files and drawn by the view (1 under symbols they lay over)")
	if r.Report.Layout.PicturesUnderlaid != 1 {
		t.Errorf("underlaid = %d, want 1", r.Report.Layout.PicturesUnderlaid)
	}
}

// A picture pasted after one that lies over an element symbol, overlapping that picture
// but no element symbol, is drawn over the element symbols too, so it keeps its place
// over the earlier picture; one an element symbol drawn after it covers cannot be, and is reported.
func TestPictureOverPictureKeepsItsOrder(t *testing.T) {
	patch := `
  <mdElement elementClass="ImageShape" xmi:id="_sym_patch">
    <geometry>380, 140, 40, 40</geometry>
    <image>` + cameoHex(plantPNG) + `</image>
  </mdElement>
`
	streams := map[string]string{}
	for k, v := range figureStreams {
		streams[k] = v
	}
	streams["BINARY-modes"] = strings.Replace(streams["BINARY-modes"], "</mdOwnedViews>", patch+"</mdOwnedViews>", 1)
	r, err := migrate.Migrate("figures.mdzip", mdzip(t, streams))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantClean(t, "figures.sysml", r)
	wantInOrder(t, "pictures in stream order, both over the element symbols", string(r.Notation),
		`@DiagramLayout::Picture { location = "images/Plant_from_the_north.png"; x = 350; y = 120; width = 40; height = 40; above = true; }`,
		`@DiagramLayout::Picture { location = "images/Plant_from_the_north.png"; x = 380; y = 140; width = 40; height = 40; above = true; }`)
	if r.Report.Layout.PicturesUnderlaid != 0 {
		t.Errorf("underlaid = %d, want none", r.Report.Layout.PicturesUnderlaid)
	}

	// Drawn after the patch, the note lies over it in the tool: the patch goes under the
	// element symbols, so the sticker over them covers it, which the report says.
	modes := figureStreams["BINARY-modes"]
	note := modes[strings.Index(modes, `<mdElement elementClass="Note" xmi:id="_sym_note">`):]
	note = note[:strings.Index(note, `<mdElement elementClass="Note" xmi:id="_sym_note_start">`)]
	modes = strings.Replace(modes, note, "", 1)
	note = strings.Replace(note, "<geometry>40, 200, 150, 40</geometry>", "<geometry>395, 150, 40, 40</geometry>", 1)
	streams["BINARY-modes"] = strings.Replace(modes, "</mdOwnedViews>", patch+note+"\n</mdOwnedViews>", 1)
	if r, err = migrate.Migrate("figures.mdzip", mdzip(t, streams)); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantClean(t, "figures.sysml", r)
	if !strings.Contains(string(r.Notation), `x = 380; y = 140; width = 40; height = 40; }`) {
		t.Errorf("the covered patch is not drawn under the element symbols:\n%s", r.Notation)
	}
	wantInOrder(t, "modes entry", reportText(t, r),
		"_diag_modes", "3 pasted images written as images/Plant_from_the_north.png, 1 pasted image drawn under the 1 pasted image it lay over, since symbols drawn after it lie over it")
	if r.Report.Layout.PicturesUnderlaid != 1 {
		t.Errorf("underlaid = %d, want 1", r.Report.Layout.PicturesUnderlaid)
	}
}

// Pasted images the tool wrote without an xmi:id are told apart all the same: one
// whose bytes do not read is reported, and the one after it is written and placed.
func TestUnnamedPicturesAreToldApart(t *testing.T) {
	patch := `
  <mdElement elementClass="ImageShape">
    <geometry>380, 140, 40, 40</geometry>
    <image>zz</image>
  </mdElement>
  <mdElement elementClass="ImageShape">
    <geometry>420, 140, 40, 40</geometry>
    <image>` + cameoHex(plantPNG) + `</image>
  </mdElement>
`
	streams := map[string]string{}
	for k, v := range figureStreams {
		streams[k] = v
	}
	streams["BINARY-modes"] = strings.Replace(streams["BINARY-modes"], "</mdOwnedViews>", patch+"</mdOwnedViews>", 1)
	r, err := migrate.Migrate("figures.mdzip", mdzip(t, streams))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantClean(t, "figures.sysml", r)
	if !strings.Contains(string(r.Notation), `@DiagramLayout::Picture { location = "images/Plant_from_the_north.png"; x = 420; y = 140; width = 40; height = 40; }`) {
		t.Errorf("the unnamed picture after the unreadable one is not drawn:\n%s", r.Notation)
	}
	if strings.Contains(string(r.Notation), "x = 380; y = 140;") {
		t.Errorf("the unreadable unnamed picture is placed:\n%s", r.Notation)
	}
	wantInOrder(t, "modes entry", reportText(t, r),
		"_diag_modes", "3 pasted images written as images/Plant_from_the_north.png, 3 pasted images not written: ",
		`the pasted image of symbol _sym_torn has bytes that do not read (octet 19 is "xx", not a hexadecimal byte)`,
		`the pasted image has bytes that do not read (octet 0 is "zz", not a hexadecimal byte)`)
	if s := r.Report.Layout; s.Pictures != 8 || s.PicturesWritten != 4 {
		t.Errorf("pictures = %d, written = %d; want 8 and 4", s.Pictures, s.PicturesWritten)
	}
}

// A pasted image whose geometry has no area — a zero or negative side — is not
// written: a Picture needs a box to fill, and the report says why it has none.
func TestPictureWithoutAreaIsNotWritten(t *testing.T) {
	streams := map[string]string{}
	for k, v := range figureStreams {
		streams[k] = v
	}
	streams["BINARY-modes"] = strings.Replace(streams["BINARY-modes"], "<geometry>350, 120, 40, 40</geometry>", "<geometry>350, 120, 0, -40</geometry>", 1)
	r, err := migrate.Migrate("figures.mdzip", mdzip(t, streams))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantClean(t, "figures.sysml", r)
	if strings.Contains(string(r.Notation), "x = 350; y = 120;") {
		t.Errorf("a picture without area is placed:\n%s", r.Notation)
	}
	wantInOrder(t, "modes entry", reportText(t, r),
		"_diag_modes", "1 pasted image written as images/Plant_from_the_north.png, 3 pasted images not written: ",
		"the pasted image of symbol _sym_sticker has no area to fill (0 by -40)")
	if s := r.Report.Layout; s.Pictures != 6 || s.PicturesWritten != 2 {
		t.Errorf("pictures = %d, written = %d; want 6 and 2", s.Pictures, s.PicturesWritten)
	}
}

// A picture pasted on a table diagram reaches the migrated view, whose table
// rendering keeps its rows and says where the picture would have been.
func TestMigratedTablePictureIsNoticed(t *testing.T) {
	r, err := migrate.Migrate("figures.mdzip", mdzip(t, figureStreams))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	rendering, err := session(t, r).ViewRendering("Plant::Roster")
	if err != nil {
		t.Fatalf("ViewRendering: %v", err)
	}
	notice := "1 picture(s) not drawn: images/Plant_from_the_north.png at (300, 20) size 120×80; a table rendering draws no picture"
	if len(rendering.Pictures) != 0 || len(rendering.Rows) != 1 || len(rendering.Notices) != 1 || rendering.Notices[0] != notice {
		t.Fatalf("table rendering = %d picture(s), %d row(s), notices %q; want none, 1 and [%q]",
			len(rendering.Pictures), len(rendering.Rows), rendering.Notices, notice)
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
	if s == nil || s.DiagramsJoined != 1 || s.StreamDiagrams != 6 || s.StreamSupplemented != 1 {
		t.Fatalf("layout summary = %+v; want 1 diagram joined and supplemented, 6 from streams", s)
	}
	report := reportText(t, r)
	wantInOrder(t, "sources", report,
		"1 joined views supplemented from their own symbol stream",
		"_diag_partial", "laid out from the diagram's own symbol stream")
	wantInOrder(t, "joined source", report,
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
