package view

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// renderWithSites renders a fixture view with the declaration sites resolved,
// as the CLI, REPL and LSP resolve them, so the cells carry exact qualified
// names and `file:line` links.
func renderWithSites(t *testing.T, file, view string) (*Rendering, Options) {
	t.Helper()
	r, idx := loadFixtures(t, file)
	rendering, err := r.Render(lookup(t, idx, view))
	if err != nil {
		t.Fatalf("render %s of %s: %v", view, file, err)
	}
	sites := r.Sites(FileLocator(r.model, func(doc string) *source.LineIndex {
		content, err := os.ReadFile(filepath.Join("testdata", doc))
		if err != nil {
			return nil
		}
		return source.NewLineIndex(content)
	}))
	return rendering, Options{Links: Links{Sites: sites}}
}

// TestDrawioGolden writes every graph-shaped PlantUML case in the draw.io form
// and checks it against its golden and what draw.io needs of a file.
func TestDrawioGolden(t *testing.T) {
	for _, c := range plantumlGoldenCases {
		if !c.kind.SupportsForm(FormDrawio) {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			rendering, options := renderWithSites(t, c.file, c.view)
			if rendering.Kind != c.kind {
				t.Fatalf("kind = %s, want %s", rendering.Kind, c.kind)
			}
			got, err := rendering.WriteWith(FormDrawio, options)
			if err != nil {
				t.Fatal(err)
			}
			direct, err := rendering.DrawioWith(options)
			if err != nil || direct != got {
				t.Fatalf("DrawioWith differs from WriteWith(FormDrawio): %v", err)
			}
			if !strings.Contains(got, ` link="`) {
				t.Errorf("no cell links its declaration")
			}
			checkGolden(t, filepath.Join("testdata", c.name+".drawio.golden"), got)
			checkDrawioDocument(t, got)
		})
	}
}

// drawioCell is one cell as the well-formedness checks read it back.
type drawioCell struct {
	id, parent, source, target string
	vertex, edge, geometry     bool
	fqn, link                  string
}

// parseDrawio reads a draw.io file back with a full xml.Decoder parse, failing
// on malformed XML, and lists its cells.
func parseDrawio(t *testing.T, doc string) []drawioCell {
	t.Helper()
	decoder := xml.NewDecoder(strings.NewReader(doc))
	var cells []drawioCell
	var open []*drawioCell
	var objects []map[string]string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("malformed draw.io XML: %v", err)
		}
		switch tok := token.(type) {
		case xml.StartElement:
			attrs := map[string]string{}
			for _, a := range tok.Attr {
				attrs[a.Name.Local] = a.Value
			}
			switch tok.Name.Local {
			case "object":
				objects = append(objects, attrs)
				if attrs["view"] == "" || attrs["kind"] == "" {
					t.Errorf("object %s lacks a view or kind attribute", attrs["id"])
				}
			case "mxCell":
				cell := &drawioCell{id: attrs["id"], parent: attrs["parent"], source: attrs["source"], target: attrs["target"],
					vertex: attrs["vertex"] == "1", edge: attrs["edge"] == "1"}
				if len(objects) > 0 {
					object := objects[len(objects)-1]
					cell.fqn, cell.link, cell.id = object["fqn"], object["link"], object["id"]
				}
				cells = append(cells, *cell)
				open = append(open, &cells[len(cells)-1])
			case "mxGeometry":
				if len(open) > 0 {
					open[len(open)-1].geometry = true
				}
			}
		case xml.EndElement:
			switch tok.Name.Local {
			case "object":
				objects = objects[:len(objects)-1]
			case "mxCell":
				open = open[:len(open)-1]
			}
		}
	}
	return cells
}

// checkDrawioDocument checks what draw.io needs of a file: well-formed XML,
// the two root cells, a geometry on every vertex, both ends on every edge, a
// parent every cell has, and one declaration per fqn: an element drawn in two
// cells (an inherited feature under each subtype) repeats its fqn, two
// declarations never share one.
func checkDrawioDocument(t *testing.T, doc string) {
	t.Helper()
	if !strings.HasPrefix(doc, `<?xml version="1.0" encoding="UTF-8"?>`+"\n") {
		t.Errorf("the file does not open with the XML declaration")
	}
	if !strings.Contains(doc, `<mxfile host="OpenSysML"`) {
		t.Errorf("the file has no <mxfile host=\"OpenSysML\"> root")
	}
	cells := parseDrawio(t, doc)
	ids := map[string]bool{}
	fqns := map[string]drawioCell{}
	for _, cell := range cells {
		if ids[cell.id] {
			t.Errorf("cell id %q written twice", cell.id)
		}
		ids[cell.id] = true
	}
	if !ids["0"] || !ids["1"] || len(cells) < 2 || cells[0].id != "0" || cells[1].id != "1" || cells[1].parent != "0" {
		t.Errorf("the model does not open with the root cells 0 and 1")
	}
	for _, cell := range cells[2:] {
		if !ids[cell.parent] {
			t.Errorf("cell %s has parent %q, which is no cell", cell.id, cell.parent)
		}
		if cell.vertex && !cell.geometry {
			t.Errorf("vertex %s has no geometry", cell.id)
		}
		if cell.edge && (!ids[cell.source] || !ids[cell.target]) {
			t.Errorf("edge %s has source %q and target %q; both must be cells", cell.id, cell.source, cell.target)
		}
		if cell.vertex == cell.edge {
			t.Errorf("cell %s is neither a vertex nor an edge", cell.id)
		}
		if cell.fqn != "" {
			if other, ok := fqns[cell.fqn]; ok && other.link != cell.link {
				t.Errorf("fqn %q on cells %s (%s) and %s (%s); an fqn names one declaration", cell.fqn, other.id, other.link, cell.id, cell.link)
			}
			fqns[cell.fqn] = cell
		}
	}
}

func TestDrawioRefusesOtherKinds(t *testing.T) {
	rendering := render(t, "sequence.sysml", "SequenceViews::pubSubView")
	if _, err := rendering.Write(FormDrawio); err == nil {
		t.Fatalf("a sequence rendering wrote draw.io; mxGraph has no lifeline grammar")
	}
}

func TestDrawioBoxRelativeTo(t *testing.T) {
	parent := drawioBox{X: 100, Y: 50, Width: 300, Height: 200}
	child := drawioBox{X: 120, Y: 90, Width: 80, Height: 40}
	got := child.relativeTo(parent)
	want := drawioBox{X: 20, Y: 40, Width: 80, Height: 40}
	if got != want {
		t.Errorf("relativeTo = %+v, want %+v", got, want)
	}
	if back := (drawioBox{X: got.X + parent.X, Y: got.Y + parent.Y, Width: got.Width, Height: got.Height}); back != child {
		t.Errorf("the conversion does not invert: %+v", back)
	}
}

func TestDrawioGrid(t *testing.T) {
	sizes := []drawioBox{{Width: 100, Height: 40}, {Width: 60, Height: 80}, {Width: 120, Height: 30}, {Width: 50, Height: 50}, {Width: 70, Height: 20}}
	boxes := drawioGrid(sizes, drawioBox{X: 10, Y: 20}, 10)
	want := []drawioBox{{10, 20, 100, 40}, {120, 20, 60, 80}, {190, 20, 120, 30}, {10, 110, 50, 50}, {70, 110, 70, 20}}
	for i := range want {
		if boxes[i] != want[i] {
			t.Errorf("box %d = %+v, want %+v", i, boxes[i], want[i])
		}
	}
	if len(drawioGrid(nil, drawioBox{}, 10)) != 0 {
		t.Errorf("an empty grid has boxes")
	}
}

func TestDrawioStyleMapping(t *testing.T) {
	style := &Style{Fill: "#ffeeaa", Line: "#112233", Text: "#445566", Font: "Fira Sans", FontSize: 11, Bold: true, Italic: true}
	got := drawioStyleOf(style, "#000000")
	want := "fillColor=#ffeeaa;strokeColor=#112233;fontColor=#445566;fontFamily=Fira Sans;fontSize=11;fontStyle=3;"
	if got != want {
		t.Errorf("drawioStyleOf = %q, want %q", got, want)
	}
	if got := drawioStyleOf(nil, "#abcdef"); got != "fillColor=#abcdef;" {
		t.Errorf("palette fill = %q", got)
	}
	if got := drawioStyleOf(&Style{Bold: true}, ""); got != "fontStyle=1;" {
		t.Errorf("bold alone = %q", got)
	}
	if got := drawioStyleOf(&Style{Italic: true}, ""); got != "fontStyle=2;" {
		t.Errorf("italic alone = %q", got)
	}
}

func TestDrawioEveryEdgeKindHasAStyle(t *testing.T) {
	for kind := EdgeKind(0); kind < edgeKindCount; kind++ {
		style := drawioEdgeStyle(kind)
		if !strings.Contains(style, "Arrow=") {
			t.Errorf("%s: style %q names no arrow", kind, style)
		}
		for _, part := range strings.Split(style, ";") {
			if !strings.Contains(part, "=") {
				t.Errorf("%s: %q is not key=value", kind, part)
			}
		}
	}
}

func TestDrawioEscapesAttributes(t *testing.T) {
	rendering := &Rendering{View: `Views::quoted "view" <1>`, Kind: KindTree,
		Roots: []*Node{{ID: "n0", Kind: "part def", Name: `A&B "quoted" <x>`}}}
	got, err := rendering.Write(FormDrawio)
	if err != nil {
		t.Fatal(err)
	}
	checkDrawioDocument(t, got)
	if !strings.Contains(got, `label="«part def»&#xA;A&amp;B &#34;quoted&#34; &lt;x&gt;"`) {
		t.Errorf("the label is not escaped by encoding/xml:\n%s", got)
	}
}

func TestDrawioFormIsWired(t *testing.T) {
	if !containsForm(Forms(), FormDrawio) || !containsForm(ViewForms(), FormDrawio) {
		t.Errorf("FormDrawio is missing from Forms or ViewForms")
	}
	if containsForm(DiagramForms(), FormDrawio) {
		t.Errorf("FormDrawio is a DiagramForm; documents have no draw.io rasterizer yet")
	}
	for _, kind := range []Kind{KindTree, KindInterconnection, KindState, KindAction, KindCase, KindMixed, KindRequirement, KindDefinition, KindPackage} {
		if !kind.SupportsForm(FormDrawio) {
			t.Errorf("%s does not support drawio", kind)
		}
	}
	for _, kind := range []Kind{KindSequence, KindTimeline, KindTable, KindMatrix, KindTextual, KindGeometry} {
		if kind.SupportsForm(FormDrawio) {
			t.Errorf("%s supports drawio", kind)
		}
	}
	if ext := fmt.Sprint(FormDrawio); ext != "drawio" {
		t.Errorf("form name %q", ext)
	}
}

func containsForm(forms []Form, form Form) bool {
	for _, f := range forms {
		if f == form {
			return true
		}
	}
	return false
}
