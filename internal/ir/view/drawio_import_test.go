package view

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// drawioGoldenImport imports a draw.io document back against the rendering
// its golden case was exported from.
func drawioGoldenImport(t *testing.T, c struct {
	name string
	file string
	view string
	kind Kind
}, document string) *DrawioImport {
	t.Helper()
	rendering, options := renderWithSites(t, c.file, c.view)
	file, err := ReadDrawio([]byte(document), c.view)
	if err != nil {
		t.Fatalf("read %s: %v", c.name, err)
	}
	imp, err := rendering.ImportDrawio(file, options)
	if err != nil {
		t.Fatalf("import %s: %v", c.name, err)
	}
	return imp
}

// TestDrawioImportUntouchedGoldenIsNoEdit is the round trip's contract: a file
// the writer emitted, imported as is, changes nothing and reports nothing.
func TestDrawioImportUntouchedGoldenIsNoEdit(t *testing.T) {
	for _, c := range plantumlGoldenCases {
		if !c.kind.SupportsForm(FormDrawio) {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			golden, err := os.ReadFile(filepath.Join("testdata", c.name+".drawio.golden"))
			if err != nil {
				t.Fatal(err)
			}
			imp := drawioGoldenImport(t, c, string(golden))
			if len(imp.Edits) != 0 {
				t.Errorf("edits = %+v, want none", imp.Edits)
			}
			if len(imp.Unmatched)+len(imp.Unknown)+len(imp.Deleted)+len(imp.Notices) != 0 {
				t.Errorf("unmatched %v unknown %d deleted %v notices %v, want nothing reported", imp.Unmatched, len(imp.Unknown), imp.Deleted, imp.Notices)
			}
		})
	}
}

// layoutCase is the placed interconnection view with a Layout, a Route, a
// Canvas, a Style, a note and a picture: every annotation the import writes.
var layoutCase = plantumlGoldenCases[7]

// drawioCellXML is the <mxCell> of the cell id, with its geometry.
func drawioCellXML(t *testing.T, document, id string) (whole, inner string) {
	t.Helper()
	re := regexp.MustCompile(`(?s)<object id="` + regexp.QuoteMeta(id) + `"[^>]*>\s*(<mxCell.*?</mxCell>)\s*</object>|<mxCell id="` + regexp.QuoteMeta(id) + `".*?(?:</mxCell>|/>)`)
	m := re.FindStringSubmatch(document)
	if m == nil {
		t.Fatalf("no cell %s in the document", id)
	}
	if m[1] != "" {
		return m[0], m[1]
	}
	return m[0], m[0]
}

// mutateCell rewrites the cell id's <mxCell> through edit.
func mutateCell(t *testing.T, document, id string, edit func(cell string) string) string {
	t.Helper()
	whole, inner := drawioCellXML(t, document, id)
	edited := strings.Replace(whole, inner, edit(inner), 1)
	if edited == whole {
		t.Fatalf("mutation of %s changed nothing", id)
	}
	return strings.Replace(document, whole, edited, 1)
}

// setGeometryAttr sets one attribute of the cell's own mxGeometry.
func setGeometryAttr(t *testing.T, cell, name, value string) string {
	t.Helper()
	re := regexp.MustCompile(`<mxGeometry\b[^>]*`)
	loc := re.FindStringIndex(cell)
	if loc == nil {
		t.Fatalf("no geometry in %s", cell)
	}
	geometry := cell[loc[0]:loc[1]]
	attr := regexp.MustCompile(` ` + name + `="[^"]*"`)
	if attr.MatchString(geometry) {
		geometry = attr.ReplaceAllString(geometry, ` `+name+`="`+value+`"`)
	} else {
		geometry = strings.Replace(geometry, "<mxGeometry", `<mxGeometry `+name+`="`+value+`"`, 1)
	}
	return cell[:loc[0]] + geometry + cell[loc[1]:]
}

func onlyEdit(t *testing.T, imp *DrawioImport, kind DrawioEditKind) DrawioEdit {
	t.Helper()
	if len(imp.Edits) != 1 {
		t.Fatalf("edits = %+v, want one %s", imp.Edits, kind)
	}
	if imp.Edits[0].Kind != kind {
		t.Fatalf("edit kind = %s, want %s", imp.Edits[0].Kind, kind)
	}
	return imp.Edits[0]
}

func TestDrawioImportMovedNodeIsALayout(t *testing.T) {
	golden, _ := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	document := mutateCell(t, string(golden), "n2", func(cell string) string {
		return setGeometryAttr(t, setGeometryAttr(t, cell, "x", "533"), "y", "71.5")
	})
	edit := onlyEdit(t, drawioGoldenImport(t, layoutCase, document), DrawioEditLayout)
	// The tank sits in the loop drawn at (40, 160); a Layout states absolute coordinates.
	if edit.FQN != "Plant::Loop::tank" || edit.Layout.X != 573 || edit.Layout.Y != 231.5 || edit.Layout.Collapsed {
		t.Errorf("layout = %s %+v", edit.FQN, edit.Layout)
	}
	if edit.Origin.Doc != "layout.sysml" {
		t.Errorf("origin = %+v, want the tank's declaration", edit.Origin)
	}
}

func TestDrawioImportMoveWithinToleranceIsNoEdit(t *testing.T) {
	golden, _ := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	_, inner := drawioCellXML(t, string(golden), "n2")
	x := regexp.MustCompile(`<mxGeometry x="([\d.]+)"`).FindStringSubmatch(inner)
	if x == nil {
		t.Fatalf("tank has no x: %s", inner)
	}
	document := mutateCell(t, string(golden), "n2", func(cell string) string {
		return setGeometryAttr(t, cell, "x", x[1]+".4")
	})
	if imp := drawioGoldenImport(t, layoutCase, document); len(imp.Edits) != 0 {
		t.Errorf("edits = %+v, want none for a move under half a pixel", imp.Edits)
	}
}

func TestDrawioImportResizedNodeKeepsItsSize(t *testing.T) {
	golden, _ := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	document := mutateCell(t, string(golden), "n2", func(cell string) string {
		return setGeometryAttr(t, cell, "width", "260")
	})
	edit := onlyEdit(t, drawioGoldenImport(t, layoutCase, document), DrawioEditLayout)
	if !edit.Layout.HasSize || edit.Layout.Width != 260 || edit.Layout.Height == 0 {
		t.Errorf("layout = %+v, want width 260 with the height kept", edit.Layout)
	}
}

func TestDrawioImportNestedChildResolvesAbsoluteCoordinates(t *testing.T) {
	golden, _ := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	file, err := ReadDrawio(golden, layoutCase.view)
	if err != nil {
		t.Fatal(err)
	}
	loop, pump := file.Cell("n0"), file.Cell("n1")
	if loop == nil || pump == nil || pump.Parent != "n0" {
		t.Fatalf("pump is not drawn inside the loop: %+v", pump)
	}
	_, inner := drawioCellXML(t, string(golden), "n1")
	rel := regexp.MustCompile(`<mxGeometry x="([\d.]+)" y="([\d.]+)"`).FindStringSubmatch(inner)
	if rel == nil {
		t.Fatalf("pump has no relative geometry: %s", inner)
	}
	x, _ := strconv.ParseFloat(rel[1], 64)
	y, _ := strconv.ParseFloat(rel[2], 64)
	if pump.Geometry.X != loop.Geometry.X+x || pump.Geometry.Y != loop.Geometry.Y+y {
		t.Errorf("pump at %+v, want the loop's %+v offset by (%v, %v)", pump.Geometry, loop.Geometry, x, y)
	}
	if port := file.Cell("n1:n1.0"); port == nil || !port.Relative || port.Geometry.X != 0.5 {
		t.Errorf("port = %+v, want its relative geometry kept", port)
	}
}

func TestDrawioImportReroutedEdgeIsARoute(t *testing.T) {
	golden, _ := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	document := mutateCell(t, string(golden), "e0", func(cell string) string {
		return regexp.MustCompile(`(?s)<Array as="points">.*?</Array>`).ReplaceAllString(cell,
			`<Array as="points"><mxPoint x="300" y="40"/><mxPoint x="420" y="40"/></Array>`)
	})
	edit := onlyEdit(t, drawioGoldenImport(t, layoutCase, document), DrawioEditRoute)
	want := []semantics.Waypoint{{X: 300, Y: 40}, {X: 420, Y: 40}}
	if edit.Route == nil || len(edit.Route.Points) != 2 || edit.Route.Points[0] != want[0] || edit.Route.Points[1] != want[1] {
		t.Errorf("route = %+v, want %v", edit.Route, want)
	}
	if edit.Edge == nil || edit.Edge.From != "Plant::Pump::outlet" || edit.Edge.To != "Plant::Tank::inlet" {
		t.Errorf("edge key = %+v", edit.Edge)
	}
}

func TestDrawioImportStraightenedEdgeClearsItsRoute(t *testing.T) {
	golden, _ := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	document := mutateCell(t, string(golden), "e0", func(cell string) string {
		return regexp.MustCompile(`(?s)<Array as="points">.*?</Array>`).ReplaceAllString(cell, "")
	})
	if edit := onlyEdit(t, drawioGoldenImport(t, layoutCase, document), DrawioEditRoute); edit.Route != nil {
		t.Errorf("route = %+v, want nil to clear", edit.Route)
	}
}

func TestDrawioImportRecolouredNodeIsAStyle(t *testing.T) {
	golden, _ := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	document := mutateCell(t, string(golden), "n2", func(cell string) string {
		return strings.Replace(cell, `style="rounded=1;`, `style="rounded=1;fillColor=#ffcc00;`, 1)
	})
	edit := onlyEdit(t, drawioGoldenImport(t, layoutCase, document), DrawioEditStyle)
	if edit.Style == nil || edit.Style.Fill != "#FFCC00" {
		t.Errorf("style = %+v, want fill #FFCC00", edit.Style)
	}
}

func TestDrawioImportResizedPageIsACanvas(t *testing.T) {
	golden, _ := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	document := strings.Replace(string(golden), `pageWidth="1200" pageHeight="800"`, `pageWidth="1400" pageHeight="900"`, 1)
	edit := onlyEdit(t, drawioGoldenImport(t, layoutCase, document), DrawioEditCanvas)
	if edit.Canvas == nil || edit.Canvas.Width != 1400 || edit.Canvas.Height != 900 || edit.Canvas.Unit != "px" {
		t.Errorf("canvas = %+v", edit.Canvas)
	}
}

func TestDrawioImportMovedNoteIsANote(t *testing.T) {
	golden, _ := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	document := mutateCell(t, string(golden), "note0", func(cell string) string {
		return setGeometryAttr(t, cell, "y", "333")
	})
	edit := onlyEdit(t, drawioGoldenImport(t, layoutCase, document), DrawioEditNote)
	if edit.Note == nil || edit.Note.Y != 333 || edit.Note.Text == "" || edit.Origin.Doc == "" {
		t.Errorf("note = %+v at %+v", edit.Note, edit.Origin)
	}
}

func TestDrawioImportMovedPictureIsAPicture(t *testing.T) {
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "local.png"), imageData.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	origin := Origin{Doc: "pictured.sysml", Span: source.Span{Offset: 7, Len: 40}}
	rendering := &Rendering{View: "Views::pictured", Kind: KindTree, Roots: []*Node{{ID: "n", Kind: "part", Name: "part"}},
		Pictures: []Picture{
			{Location: "local.png", Dir: dir, X: 10, Y: 20, Width: 40, Height: 30},
			{Location: "local.png", Dir: dir, X: 60, Y: 20, Width: 40, Height: 30, Above: true, Origin: origin},
		}}
	exported, err := rendering.Drawio()
	if err != nil {
		t.Fatal(err)
	}
	document := mutateCell(t, exported, "picture1", func(cell string) string {
		return setGeometryAttr(t, cell, "x", "715")
	})
	file, err := ReadDrawio([]byte(document), "")
	if err != nil {
		t.Fatal(err)
	}
	imp, err := rendering.ImportDrawio(file, Options{})
	if err != nil {
		t.Fatal(err)
	}
	edit := onlyEdit(t, imp, DrawioEditPicture)
	if edit.Picture == nil || edit.Picture.X != 715 || edit.Picture.Y != 20 || edit.Picture.Location != "local.png" || !edit.Picture.Above || edit.Origin != origin || edit.Index != 1 {
		t.Errorf("picture = %+v at %+v", edit.Picture, edit.Origin)
	}
}

func TestDrawioImportReportsWhatItCannotImport(t *testing.T) {
	golden, _ := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	document := strings.Replace(string(golden), `fqn="Plant::Loop::tank"`, `fqn="Plant::Loop::reservoir"`, 1)
	document = strings.Replace(document, "</root>", `<mxCell id="hand" value="todo" style="rounded=1" vertex="1" parent="1"><mxGeometry x="900" y="700" width="80" height="40" as="geometry"/></mxCell></root>`, 1)
	imp := drawioGoldenImport(t, layoutCase, document)
	if len(imp.Unmatched) != 1 || imp.Unmatched[0] != "Plant::Loop::reservoir" {
		t.Errorf("unmatched = %v", imp.Unmatched)
	}
	if len(imp.Deleted) != 1 || imp.Deleted[0] != "Plant::Loop::tank" {
		t.Errorf("deleted = %v", imp.Deleted)
	}
	if len(imp.Unknown) != 1 || imp.Unknown[0].ID != "hand" {
		t.Errorf("unknown = %+v", imp.Unknown)
	}
	if len(imp.Edits) != 0 {
		t.Errorf("edits = %+v, want none", imp.Edits)
	}
}
