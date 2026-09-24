package migrate_test

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

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

// A frame alone is a blank diagram; a picture and a text box stand for no
// model element; a symbol naming an element shows it although the tool's
// usedElements list omits it, or lists another element only.
var figureStreams = map[string]string{
	"BINARY-blank": strings.Replace(streamHead, "%s", "_diag_blank", 1) + `
</mdOwnedViews>`,
	"BINARY-poster": strings.Replace(streamHead, "%s", "_diag_poster", 1) + `
  <mdElement elementClass="ImageShape" xmi:id="_photo">
    <geometry>40, 40, 300, 200</geometry>
    <image>iVBORw0KGgo=</image>
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
// omits it; and a diagram with no stream is left out with what is known.
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
	wantInOrder(t, "pictures section", notation,
		"part Pictures : DocumentQueries::Section {",
		`attribute redefines text = "Nothing to see";`,
		`attribute redefines text = "The plant, photographed";`,
		"part diagram : DocumentQueries::Diagram {",
		"ref redefines source = Plant::Unlisted;")
	for _, name := range []string{"Blank", "Poster", "Silent"} {
		if strings.Contains(notation, "source = Plant::"+name+";") {
			t.Errorf("the empty diagram %s is drawn:\n%s", name, notation)
		}
	}
	var es []migrate.Entry
	for _, e := range entriesFor(r, "_st_pictures_image") {
		if !strings.HasPrefix(e.Note, "the paragraph is the caption") {
			es = append(es, e)
		}
	}
	if len(es) != 4 {
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
		},
		migrate.Approximated: {
			": no Diagram shows the SysML Block Definition Diagram 'Poster': it shows no model element, only an image and a text box standing for none, and its view exposes nothing, so the figure would be empty and is left out; its caption stands alone",
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
		"*Pump Modes*", "```mermaid", "Idle", "Running", "How the pump behaves",
		"*Priming*", "```mermaid", "Fill", "Vent", "How the pump is primed",
		"## Pictures",
		"Nothing to see",
		"The plant, photographed",
		"*Unlisted*", "```mermaid", "Plant::Unlisted — tree rendering", `"Tank<br>«part def»"`, `"Pump<br>«part def»"`)
	if n := strings.Count(md, "```mermaid"); n != 3 {
		t.Errorf("Plant Handbook draws %d figures, want 3:\n%s", n, md)
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
// figure over them is left out with that reason, and a collection over the
// diagram listing one element is refused for the stream it names.
func TestFiguresWithoutStreams(t *testing.T) {
	r := migrateFixtureFile(t, "figures")
	wantNote(t, r, "_st_shown_collect", migrate.Unmapped,
		"it collects what the SysML Block Definition Diagram 'Partial' shows beyond the 1 element the tool lists, whose symbols cannot be read and what the SysML Block Definition Diagram 'Unlisted' shows, which the archive does not record")
	for _, name := range []string{"Blank", "Poster", "Unlisted", "Silent"} {
		if strings.Contains(string(r.Notation), "source = Plant::"+name+";") {
			t.Errorf("the diagram %s, whose content is unread, is drawn:\n%s", name, r.Notation)
		}
	}
	n := 0
	for _, e := range entriesFor(r, "_st_pictures_image") {
		if strings.HasPrefix(e.Note, "the paragraph is the caption") {
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
