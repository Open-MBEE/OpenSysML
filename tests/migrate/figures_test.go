package migrate_test

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// plantPNG is a real 12×8 PNG, the picture the fixture's diagrams carry pasted.
//
//go:embed testdata/xmi/plant.png
var plantPNG []byte

// cameoHex writes bytes as MagicDraw serializes a pasted image: lowercase
// hexadecimal octets without zero padding, separated by spaces.
func cameoHex(data []byte) string {
	octets := make([]string, len(data))
	for i, b := range data {
		octets[i] = strconv.FormatInt(int64(b), 16)
	}
	return strings.Join(octets, " ")
}

// mdzip zips the figures fixture as MagicDraw does, each diagram's symbols in
// the stream entry its binaryObject names.
func mdzip(t *testing.T, streams map[string]string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/xmi/figures.xmi")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entries := map[string][]byte{
		"com.nomagic.ci.metamodel.project":      []byte("<?xml version=\"1.0\"?><project/>"),
		"com.nomagic.magicdraw.uml_model.model": data,
	}
	for name, stream := range streams {
		entries[name] = []byte(stream)
	}
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const streamHead = `<?xml version="1.0" encoding="UTF-8"?>
<mdOwnedViews xmlns:xmi="http://www.omg.org/spec/XMI/20131001">
  <mdElement elementClass="DiagramFrame" xmi:id="_frame">
    <elementID xmi:idref="%s"/>
    <geometry>10, 10, 800, 600</geometry>
  </mdElement>`

// A frame alone is a blank diagram; a pasted picture and a text box stand for
// no model element but are drawn; a symbol naming an element shows it although
// the list omits it, or lists another element only, and a listed element no
// symbol displays is not shown. The modes diagram pastes the same picture
// under its states and a sticker of it over one, names a file the archive
// lacks, and carries bytes that do not read.
var figureStreams = map[string]string{
	"BINARY-blank": strings.Replace(streamHead, "%s", "_diag_blank", 1) + `
</mdOwnedViews>`,
	"BINARY-stale": strings.Replace(streamHead, "%s", "_diag_stale", 1) + `
  <mdElement elementClass="Class" xmi:id="_sym_stale_tank">
    <elementID xmi:idref="_blk_tank"/>
    <geometry>100, 100, 120, 60</geometry>
  </mdElement>
</mdOwnedViews>`,
	"BINARY-poster": strings.Replace(streamHead, "%s", "_diag_poster", 1) + `
  <mdElement elementClass="ImageShape" xmi:id="_photo">
    <properties>
      <mdElement elementClass="FileProperty">
        <propertyID>IMAGE</propertyID>
        <value>Plant from the north.png</value>
      </mdElement>
    </properties>
    <geometry>40, 40, 300, 200</geometry>
    <image>` + cameoHex(plantPNG) + `</image>
  </mdElement>
  <mdElement elementClass="TextBox" xmi:id="_legend">
    <geometry>40, 260, 300, 40</geometry>
    <text>The plant from the north</text>
  </mdElement>
</mdOwnedViews>`,
	"BINARY-unlisted": strings.Replace(streamHead, "%s", "_diag_unlisted", 1) + `
  <mdElement elementClass="Class" xmi:id="_sym_tank">
    <elementID xmi:idref="_blk_tank"/>
    <geometry>100, 100, 120, 60</geometry>
    <mdOwnedViews>
      <mdElement elementClass="Class" xmi:id="_sym_pump">
        <elementID xmi:idref="_blk_pump"/>
      </mdElement>
    </mdOwnedViews>
  </mdElement>
</mdOwnedViews>`,
	"BINARY-modes": strings.Replace(streamHead, "%s", "_diag_modes", 1) + `
  <mdElement elementClass="ImageShape" xmi:id="_sym_backdrop">
    <geometry>20, 20, 400, 300</geometry>
    <image>` + cameoHex(plantPNG) + `</image>
  </mdElement>
  <mdElement elementClass="Pseudostate" xmi:id="_sym_init">
    <elementID xmi:idref="_sm_init"/>
    <geometry>60, 40, 16, 16</geometry>
  </mdElement>
  <mdElement elementClass="State" xmi:id="_sym_idle">
    <elementID xmi:idref="_st_idle"/>
    <properties>
      <mdElement elementClass="ColorProperty">
        <propertyID>FILL_COLOR</propertyID>
        <value xmi:value="-1973821"/>
      </mdElement>
      <mdElement elementClass="ColorProperty">
        <propertyID>PEN_COLOR</propertyID>
        <value xmi:value="-6710948"/>
      </mdElement>
      <mdElement elementClass="ColorProperty">
        <propertyID>TEXT_COLOR</propertyID>
        <value xmi:value="-16777216"/>
      </mdElement>
      <mdElement elementClass="FontProperty">
        <propertyID>FONT</propertyID>
        <fontName>Arial</fontName>
        <size xmi:value="11"/>
        <style xmi:value="1"/>
      </mdElement>
    </properties>
    <geometry>40, 100, 120, 50</geometry>
  </mdElement>
  <mdElement elementClass="State" xmi:id="_sym_running">
    <elementID xmi:idref="_st_running"/>
    <properties>
      <mdElement elementClass="BooleanProperty">
        <propertyID>USE_FILL_COLOR</propertyID>
        <value xmi:value="false"/>
      </mdElement>
    </properties>
    <geometry>260, 100, 120, 50</geometry>
  </mdElement>
  <mdElement elementClass="Transition" xmi:id="_sym_t_init">
    <elementID xmi:idref="_t_init"/>
    <linkFirstEndID xmi:idref="_sym_init"/>
    <linkSecondEndID xmi:idref="_sym_idle"/>
    <geometry>68, 56; 68, 100; </geometry>
  </mdElement>
  <mdElement elementClass="Transition" xmi:id="_sym_t_start">
    <elementID xmi:idref="_t_start"/>
    <linkFirstEndID xmi:idref="_sym_idle"/>
    <linkSecondEndID xmi:idref="_sym_running"/>
    <geometry>160, 125; 210, 125; 210, 130; 260, 130; </geometry>
  </mdElement>
  <mdElement elementClass="Note" xmi:id="_sym_note">
    <elementID xmi:idref="_cmt_pump"/>
    <geometry>40, 200, 150, 40</geometry>
  </mdElement>
  <mdElement elementClass="NoteAnchor" xmi:id="_sym_anchor">
    <linkFirstEndID xmi:idref="_sym_note"/>
    <linkSecondEndID xmi:idref="_sym_idle"/>
    <geometry>100, 200; 100, 150; </geometry>
  </mdElement>
  <mdElement elementClass="Note" xmi:id="_sym_note_start">
    <elementID xmi:idref="_cmt_start"/>
    <geometry>180, 40, 100, 30</geometry>
  </mdElement>
  <mdElement elementClass="NoteAnchor" xmi:id="_sym_anchor_start">
    <linkFirstEndID xmi:idref="_sym_note_start"/>
    <linkSecondEndID xmi:idref="_sym_t_start"/>
    <geometry>210, 70; 210, 125; </geometry>
  </mdElement>
  <mdElement elementClass="TextBox" xmi:id="_sym_draft">
    <geometry>600, 20, 80, 12</geometry>
    <text>Draft only</text>
  </mdElement>
  <mdElement elementClass="ImageShape" xmi:id="_sym_logo">
    <geometry>600, 400, 150, 150</geometry>
    <properties>
      <mdElement elementClass="StringProperty">
        <propertyID>IMAGE_FILE</propertyID>
        <value>logo.png</value>
      </mdElement>
    </properties>
  </mdElement>
  <mdElement elementClass="ImageShape" xmi:id="_sym_torn">
    <geometry>600, 200, 100, 100</geometry>
    <image>89 50 4e 47 d a 1a a 0 0 0 d 49 48 44 52 0 0 0 xx</image>
  </mdElement>
  <mdElement elementClass="ImageShape" xmi:id="_sym_sticker">
    <geometry>350, 120, 40, 40</geometry>
    <image>` + cameoHex(plantPNG) + `</image>
  </mdElement>
</mdOwnedViews>`,
	"BINARY-partial": strings.Replace(streamHead, "%s", "_diag_partial", 1) + `
  <mdElement elementClass="Class" xmi:id="_sym_partial_tank">
    <elementID xmi:idref="_blk_tank"/>
    <geometry>100, 100, 120, 60</geometry>
  </mdElement>
  <mdElement elementClass="Class" xmi:id="_sym_partial_pump">
    <elementID xmi:idref="_blk_pump"/>
    <geometry>300, 100, 120, 60</geometry>
  </mdElement>
</mdOwnedViews>`,
}

// An Image over a state machine or activity diagram draws the graph-form
// view; one over a diagram the archive's stream shows to be blank, or to hold
// only a pasted picture, leaves the figure out with the reason and keeps its
// caption; a symbol the stream names is exposed although the tool's list
// omits it; a listed element no symbol displays is not exposed, so its figure
// draws the stream's symbols alone; and a diagram with no stream is left out
// with what is known.
func TestFiguresFromArchiveStreams(t *testing.T) {
	r, err := migrate.Migrate("figures.mdzip", mdzip(t, figureStreams))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantClean(t, "figures.sysml", r)
	checkGolden(t, "testdata/xmi/figures.mdzip.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/figures.mdzip.golden.report.txt", report.Bytes())
	notation := string(r.Notation)
	wantInOrder(t, "graph-form figures", notation,
		"view 'Pump Modes' : StandardViewDefinitions::StateTransitionView {",
		"view Priming : StandardViewDefinitions::ActionFlowView {",
		"part Behaviors : DocumentQueries::Section {",
		"part diagram : DocumentQueries::Diagram {",
		`attribute redefines caption = "Pump Modes";`,
		"ref redefines source = modes.'Pump Modes';",
		`attribute redefines text = "How the pump behaves";`,
		"part 'diagram 2' : DocumentQueries::Diagram {",
		`attribute redefines caption = "Priming";`,
		"ref redefines source = prime.Priming;",
		`attribute redefines text = "How the pump is primed";`)
	wantInOrder(t, "stream-read exposure", notation,
		"view Unlisted {", "expose Tank;", "expose Pump;")
	wantInOrder(t, "pasted pictures", notation,
		"view 'Pump Modes' : StandardViewDefinitions::StateTransitionView {",
		`@DiagramLayout::Picture { location = "images/Plant_from_the_north.png"; x = 20; y = 20; width = 400; height = 300; }`,
		`@DiagramLayout::Picture { location = "images/Plant_from_the_north.png"; x = 350; y = 120; width = 40; height = 40; above = true; }`,
		"view Poster {",
		`@DiagramLayout::Picture { location = "images/Plant_from_the_north.png"; x = 40; y = 40; width = 300; height = 200; alt = "Plant from the north"; }`,
		`@DiagramLayout::Note { text = "The plant from the north";`)
	if strings.Count(notation, "DiagramLayout::Picture {") != 3 {
		t.Errorf("pictures drawn = %d, want the 3 with bytes:\n%s", strings.Count(notation, "DiagramLayout::Picture {"), notation)
	}
	if got := r.Files["images/Plant_from_the_north.png"]; !bytes.Equal(got, plantPNG) || len(r.Files) != 1 {
		t.Errorf("files = %d, want the one picture the three symbols share, byte for byte", len(r.Files))
	}
	wantInOrder(t, "pictures section", notation,
		"part Pictures : DocumentQueries::Section {",
		`attribute redefines text = "Nothing to see";`,
		"part diagram : DocumentQueries::Diagram {",
		`attribute redefines caption = "Poster";`,
		"ref redefines source = Plant::Poster;",
		`attribute redefines text = "The plant, photographed";`,
		"part 'diagram 2' : DocumentQueries::Diagram {",
		"ref redefines source = Plant::Unlisted;",
		"part 'diagram 3' : DocumentQueries::Diagram {",
		"ref redefines source = Plant::Stale;")
	for _, name := range []string{"Blank", "Silent"} {
		if strings.Contains(notation, "source = Plant::"+name+";") {
			t.Errorf("the empty diagram %s is drawn:\n%s", name, notation)
		}
	}
	if stale := notation[strings.Index(notation, "view Stale {"):]; strings.Contains(stale[:strings.Index(stale, "}")], "expose Pump;") || !strings.Contains(stale[:strings.Index(stale, "}")], "expose Tank;") {
		t.Errorf("the view of Stale does not expose what its symbols display alone:\n%s", notation)
	}
	var es []migrate.Entry
	for _, e := range entriesFor(r, "_st_pictures_image") {
		if !strings.HasPrefix(e.Note, "the paragraph is the") {
			es = append(es, e)
		}
	}
	if len(es) != 5 {
		t.Fatalf("Image entries = %+v, want one per diagram", es)
	}
	notes := map[migrate.Verdict][]string{}
	for _, e := range es {
		notes[e.Verdict] = append(notes[e.Verdict], e.Target+": "+e.Note)
	}
	wantNotes := map[migrate.Verdict][]string{
		migrate.Mapped: {
			": no Diagram shows the SysML Block Definition Diagram 'Blank': it draws nothing at all, and its view exposes nothing, so the figure would be empty and is left out; its caption stands alone",
			"part 'Plant Documents'::'Plant Handbook Document'::Pictures::diagram: ",
			"part 'Plant Documents'::'Plant Handbook Document'::Pictures::'diagram 2': ",
			"part 'Plant Documents'::'Plant Handbook Document'::Pictures::'diagram 3': ",
		},
		migrate.Approximated: {
			": no Diagram shows the SysML Block Definition Diagram 'Silent': it shows no model element, and its view exposes nothing, so the figure would be empty and is left out",
		},
	}
	for verdict, want := range wantNotes {
		for _, w := range want {
			found := false
			for _, n := range notes[verdict] {
				found = found || strings.Contains(n, w)
			}
			if !found {
				t.Errorf("no %v Image entry notes %q; entries: %+v", verdict, w, es)
			}
		}
	}

	s := session(t, r)
	md := markdown(t, s, "'Plant Documents'::'Plant Handbook Document'")
	wantInOrder(t, "Plant Handbook Markdown", md,
		"# Plant Handbook",
		"## Behaviors",
		"*Pump Modes*", "```dot", "// layout: neato -n2", "Idle", "Running", "How the pump behaves",
		"*Priming*", "```mermaid", "Fill", "Vent", "How the pump is primed",
		"## Pictures",
		"Nothing to see",
		"*Poster*", "```dot", "// view: Plant::Poster", `"picture:0" [shape=none, style="", label="", image="images/Plant_from_the_north.png"`, `tooltip="Plant from the north"`,
		"The plant, photographed",
		"*Unlisted*", "```dot", "// view: Plant::Unlisted", "// layout: neato -n", "<b>Tank</b>", "«part def»", `pos="160,480!"`,
		"*Stale*", "```dot", "// view: Plant::Stale", "<b>Tank</b>", "«part def»")
	// The stream positions four of the five diagrams, so those are drawn
	// by Graphviz where they state; the unpositioned activity stays Mermaid.
	if dot, mermaid := strings.Count(md, "```dot"), strings.Count(md, "```mermaid"); dot != 4 || mermaid != 1 {
		t.Errorf("Plant Handbook draws %d dot and %d mermaid figures, want 4 and 1:\n%s", dot, mermaid, md)
	}
	if modes := md[strings.Index(md, "*Pump Modes*"):]; strings.Count(modes[:strings.Index(modes, "*Priming*")], `image="images/Plant_from_the_north.png"`) != 2 {
		t.Errorf("the figure of Pump Modes does not draw its backdrop and sticker:\n%s", modes)
	}
	if strings.Contains(md, "drawn as Mermaid") {
		t.Errorf("a Graphviz fallback notice appears where no drawer was asked:\n%s", md)
	}
	if stale := md[strings.Index(md, "*Stale*"):]; strings.Contains(stale[:strings.Index(stale, "## Shown")], "Pump") {
		t.Errorf("the figure of Stale draws the element no symbol displays:\n%s", stale)
	}
	if strings.Contains(md, "exposes nothing") || strings.Contains(md, "empty[") {
		t.Errorf("an empty figure is rendered:\n%s", md)
	}
	page := html(t, s, "'Plant Documents'::'Plant Handbook Document'")
	if strings.Contains(page, "exposes nothing") {
		t.Errorf("an empty figure is rendered as HTML:\n%s", page)
	}
	// Things on the diagrams: the stream adds the pump to the one element the
	// tool lists for Partial, so the table names both blocks.
	wantInOrder(t, "Shown Blocks query", notation,
		"calc def 'Plant Handbook Shown Blocks Rows'",
		`DocumentQueries::Named(qualifiedName = ("Plant::Tank", "Plant::Pump")),`)
	wantNote(t, r, "_st_shown_table", migrate.Mapped, "")
}

// A stream the archive names but does not hold, or holds cut short, leaves
// what its diagram shows unknown beyond the elements the tool lists, so a
// collection over it and a readable diagram is refused rather than named from
// the list and the readable one as if complete.
func TestShownCollectionOverUnreadStream(t *testing.T) {
	cut := figureStreams["BINARY-partial"]
	cut = cut[:strings.Index(cut, "_sym_partial_pump")]
	for name, partial := range map[string]string{"missing": "", "truncated": cut} {
		t.Run(name, func(t *testing.T) {
			streams := map[string]string{}
			for k, v := range figureStreams {
				streams[k] = v
			}
			delete(streams, "BINARY-partial")
			if partial != "" {
				streams["BINARY-partial"] = partial
			}
			r, err := migrate.Migrate("figures.mdzip", mdzip(t, streams))
			if err != nil {
				t.Fatalf("Migrate: %v", err)
			}
			wantClean(t, "figures.sysml", r)
			notation := string(r.Notation)
			if strings.Contains(notation, "calc def 'Plant Handbook Shown Blocks Rows'") {
				t.Errorf("a collection over a diagram whose stream is unread is spelled from its list and the readable diagram:\n%s", notation)
			}
			wantNote(t, r, "_st_shown_collect", migrate.Unmapped,
				"it collects what the SysML Block Definition Diagram 'Partial' shows beyond the 1 element the tool lists, whose symbols cannot be read")
			wantInOrder(t, "stream-read exposure", notation,
				"view Unlisted {", "expose Tank;", "expose Pump;")
		})
	}
}

// Without the archive, what the four listless diagrams show is unknown; every
// figure over them is left out with that reason, the diagram listing an element
// its stream would show to be gone is drawn from its list, and a collection over
// the diagram listing one element is refused for the stream it names.
func TestFiguresWithoutStreams(t *testing.T) {
	r := migrateFixtureFile(t, "figures")
	wantInOrder(t, "the listed diagram", string(r.Notation),
		"view Stale {", "expose Pump;", "ref redefines source = Plant::Stale;")
	wantNote(t, r, "_st_shown_collect", migrate.Unmapped,
		"it collects what the SysML Block Definition Diagram 'Partial' shows beyond the 1 element the tool lists, whose symbols cannot be read and what the SysML Block Definition Diagram 'Unlisted' shows, which the archive does not record")
	for _, name := range []string{"Blank", "Poster", "Unlisted", "Silent"} {
		if strings.Contains(string(r.Notation), "source = Plant::"+name+";") {
			t.Errorf("the diagram %s, whose content is unread, is drawn:\n%s", name, r.Notation)
		}
	}
	n := 0
	for _, e := range entriesFor(r, "_st_pictures_image") {
		if strings.HasPrefix(e.Note, "the paragraph is the caption") || strings.HasSuffix(e.Target, "::Pictures::diagram") {
			continue
		}
		n++
		if e.Verdict != migrate.Approximated || !strings.Contains(e.Note, "it shows no model element, and its view exposes nothing") {
			t.Errorf("Image entry = %+v, want Approximated noting the unread content", e)
		}
	}
	if n != 4 {
		t.Errorf("%d figures are left out, want 4", n)
	}
}
