package view

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// renderWithSites renders a fixture view with the declaration sites resolved, as the
// CLI, REPL and LSP do, so the cells carry exact qualified names and `file:line` links.
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
	fqn, link, style           string
	box                        drawioBox
	relative                   bool  // box.X and box.Y are fractions of the parent's box
	offset                     Point // added to a relative position
	points                     []Point
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
					vertex: attrs["vertex"] == "1", edge: attrs["edge"] == "1", style: attrs["style"]}
				if len(objects) > 0 {
					object := objects[len(objects)-1]
					cell.fqn, cell.link, cell.id = object["fqn"], object["link"], object["id"]
				}
				cells = append(cells, *cell)
				open = append(open, &cells[len(cells)-1])
			case "mxGeometry":
				if len(open) > 0 {
					open[len(open)-1].geometry = true
					open[len(open)-1].box = drawioBox{X: xmlNumber(t, attrs["x"]), Y: xmlNumber(t, attrs["y"]),
						Width: xmlNumber(t, attrs["width"]), Height: xmlNumber(t, attrs["height"])}
					open[len(open)-1].relative = attrs["relative"] == "1"
				}
			case "mxPoint":
				if len(open) > 0 {
					cell := open[len(open)-1]
					point := Point{X: xmlNumber(t, attrs["x"]), Y: xmlNumber(t, attrs["y"])}
					switch attrs["as"] {
					case "":
						cell.points = append(cell.points, point)
					case "offset":
						cell.offset = point
					}
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

// xmlNumber reads a numeric attribute, zero when absent.
func xmlNumber(t *testing.T, value string) float64 {
	t.Helper()
	if value == "" {
		return 0
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		t.Fatalf("attribute value %q is not a number: %v", value, err)
	}
	return n
}

// onDrawioBorder reports a point on the border of box, within a pixel.
func onDrawioBorder(p Point, box drawioBox) bool {
	inside := p.X >= box.X-1 && p.X <= box.right()+1 && p.Y >= box.Y-1 && p.Y <= box.bottom()+1
	onX := math.Abs(p.X-box.X) <= 1 || math.Abs(p.X-box.right()) <= 1
	onY := math.Abs(p.Y-box.Y) <= 1 || math.Abs(p.Y-box.bottom()) <= 1
	return inside && (onX || onY)
}

// drawioAnchor is where an edge end sits on its cell's box, read from the
// exitX/exitY or entryX/entryY fractions of the edge's style.
func drawioAnchor(t *testing.T, style, end string, box drawioBox) (Point, bool) {
	t.Helper()
	var p Point
	found := 0
	for _, entry := range strings.Split(style, ";") {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch key {
		case end + "X":
			p.X = box.X + xmlNumber(t, value)*box.Width
			found++
		case end + "Y":
			p.Y = box.Y + xmlNumber(t, value)*box.Height
			found++
		}
	}
	return p, found == 2
}

// drawioAbsolute is a cell's box in page coordinates: its parent's origin added,
// or, placed relative to the parent, the fractions of the parent's box plus the offset.
func drawioAbsolute(cells map[string]drawioCell, id string) drawioBox {
	cell := cells[id]
	box := cell.box
	if cell.parent == "" || cell.parent == "1" {
		return box
	}
	parent := drawioAbsolute(cells, cell.parent)
	if cell.relative {
		box.X = parent.X + box.X*parent.Width + cell.offset.X
		box.Y = parent.Y + box.Y*parent.Height + cell.offset.Y
		return box
	}
	box.X += parent.X
	box.Y += parent.Y
	return box
}

// checkDrawioDocument checks what draw.io needs of a file (well-formed XML, root cells,
// every vertex placed, every edge and parent resolved) and that two declarations never share an fqn.
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

// A local or data-URL picture is embedded as draw.io keeps an image in a style,
// `data:<type>,<base64>` with no `;base64`; a refused picture stays a notice.
func TestDrawioPictures(t *testing.T) {
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "local.png"), imageData.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(imageData.Bytes())
	rendering := &Rendering{View: "Views::pictured", Kind: KindTree, Roots: []*Node{{ID: "n", Kind: "part", Name: "part"}},
		Pictures: []Picture{
			{Location: "local.png", Dir: dir, X: 10, Y: 20, Width: 40, Height: 30},
			{Location: "data:image/png;base64," + encoded, X: 60, Y: 20, Width: 40, Height: 30, Above: true},
			{Location: "https://example.com/remote.png", Dir: dir, X: 110, Y: 20, Width: 20, Height: 10},
		}}
	got, err := rendering.Write(FormDrawio)
	if err != nil {
		t.Fatal(err)
	}
	checkDrawioDocument(t, got)
	for _, id := range []string{"picture0", "picture1"} {
		if !strings.Contains(got, `id="`+id+`"`) {
			t.Errorf("%s is not drawn:\n%s", id, got)
		}
	}
	if n := strings.Count(got, "image=data:image/png,"+encoded+";"); n != 2 {
		t.Errorf("%d pictures carry the image as draw.io stores it, want 2:\n%s", n, got)
	}
	if strings.Contains(got, ";base64,") {
		t.Errorf("a data URL keeps ;base64, which a style splits at:\n%s", got)
	}
	if strings.Contains(got, `id="picture2"`) || !strings.Contains(got, "remote.png") {
		t.Errorf("the remote picture is drawn or its refusal is not noted:\n%s", got)
	}
}

// A Note about several elements is one note cell anchored to each subject; Notes
// stated apart are separate cells even when their text coincides.
func TestDrawioSharesANoteAmongItsAnchors(t *testing.T) {
	rendering := render(t, "shared-notes.sysml", "SharedViews::rigView")
	got, err := rendering.Write(FormDrawio)
	if err != nil {
		t.Fatal(err)
	}
	checkDrawioDocument(t, got)
	cells := regexp.MustCompile(`<mxCell id="(note\d+)" value="([^"]*)"`).FindAllStringSubmatch(got, -1)
	byText := map[string][]string{}
	for _, m := range cells {
		byText[m[2]] = append(byText[m[2]], m[1])
	}
	if len(byText["calibrated"]) != 1 {
		t.Fatalf("the note about two parts is drawn as %v, want one cell:\n%s", byText["calibrated"], got)
	}
	if n := strings.Count(got, `id="`+byText["calibrated"][0]+`:anchor`); n != 2 {
		t.Errorf("the shared note has %d anchors, want 2:\n%s", n, got)
	}
	if len(byText["on loan"]) != 2 {
		t.Errorf("the two notes stated apart are drawn as %v, want two cells:\n%s", byText["on loan"], got)
	}
}

// TestDrawioRoutesUnroutedEdges checks that the edges the model leaves unrouted
// come out with libavoid's orthogonal waypoints: every edge of a positioned
// interconnection and of a nested tree, each inside the container it belongs
// to, while a stated Route is written as it is.
func TestDrawioRoutesUnroutedEdges(t *testing.T) {
	for _, c := range []struct{ name, file, view string }{
		{"interconnection-ports", "interconnection-ports.sysml", "ToasterViews::toasterView"},
		{"interconnection-header-ports", "interconnection-header-ports.sysml", "PumpStationViews::stationView"},
		{"tree-edges", "tree-edges.sysml", "FleetViews::structure"},
		{"layout", "layout.sysml", "PlantViews::placedView"},
	} {
		t.Run(c.name, func(t *testing.T) {
			rendering, options := renderWithSites(t, c.file, c.view)
			doc, err := rendering.DrawioWith(options)
			if err != nil {
				t.Fatal(err)
			}
			cells := map[string]drawioCell{}
			for _, cell := range parseDrawio(t, doc) {
				cells[cell.id] = cell
			}
			ids := drawioEdgeIDs(rendering.Edges)
			for _, edge := range rendering.Edges {
				cell, ok := cells[ids[[2]string{edge.From, edge.To}]]
				if !ok {
					continue
				}
				if len(edge.Route) > 0 {
					if !slices.Equal(cell.points, edge.Route) {
						t.Errorf("%s: stated route %v written as %v", cell.id, edge.Route, cell.points)
					}
					continue
				}
				exit, exitOK := drawioAnchor(t, cell.style, "exit", drawioAbsolute(cells, cell.source))
				entry, entryOK := drawioAnchor(t, cell.style, "entry", drawioAbsolute(cells, cell.target))
				if !exitOK || !entryOK || !strings.Contains(cell.style, "edgeStyle=none;") {
					t.Errorf("%s (%s -> %s): not routed: style %q", cell.id, edge.From, edge.To, cell.style)
					continue
				}
				for _, end := range []struct {
					name string
					p    Point
				}{{cell.source, exit}, {cell.target, entry}} {
					// A port cell is left through its border; a container is
					// left through its header's underside, inside its box.
					if strings.Contains(end.name, ":") && !onDrawioBorder(end.p, drawioAbsolute(cells, end.name)) {
						t.Errorf("%s: end %v is not on the border of %s", cell.id, end.p, end.name)
					}
				}
				path := append(append([]Point{exit}, cell.points...), entry)
				for j := 1; j < len(path); j++ {
					a, b := path[j-1], path[j]
					if math.Abs(a.X-b.X) > 0.5 && math.Abs(a.Y-b.Y) > 0.5 {
						t.Errorf("%s: segment %v -> %v is not orthogonal", cell.id, a, b)
					}
				}
				from, to := cells[strings.SplitN(cell.source, ":", 2)[0]], cells[strings.SplitN(cell.target, ":", 2)[0]]
				if from.parent == to.parent && from.parent != "1" {
					within := drawioAbsolute(cells, from.parent)
					for _, p := range cell.points {
						if p.X < within.X || p.Y < within.Y || p.X > within.right() || p.Y > within.bottom() {
							t.Errorf("%s: waypoint %v leaves its container %s %v", cell.id, p, from.parent, within)
						}
					}
				}
			}
		})
	}
}
