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
)

// setCellAttr sets the attribute name of the <mxCell> id to value, XML-escaped as given.
func setCellAttr(t *testing.T, document, id, name, value string) string {
	t.Helper()
	re := regexp.MustCompile(`(<mxCell id="` + regexp.QuoteMeta(id) + `"[^>]*? ` + name + `=")[^"]*(")`)
	if !re.MatchString(document) {
		t.Fatalf("no %s on cell %s", name, id)
	}
	return re.ReplaceAllString(document, "${1}"+strings.ReplaceAll(value, "$", "$$")+"${2}")
}

// setImage replaces the image= source in the style of the cell id.
func setImage(t *testing.T, document, id, src string) string {
	t.Helper()
	whole, _ := drawioCellXML(t, document, id)
	re := regexp.MustCompile(`image=[^;"]*;`)
	if !re.MatchString(whole) {
		t.Fatalf("no image on cell %s", id)
	}
	return strings.Replace(document, whole, re.ReplaceAllLiteralString(whole, "image="+src+";"), 1)
}

func drawioNoteCase(t *testing.T) (golden string, noteText string, noteY float64) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := ReadDrawio(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	note := file.Cell("note0")
	if note == nil || note.Value == "" {
		t.Fatal("the golden draws no note0")
	}
	return string(raw), note.Value, note.Geometry.Y
}

func TestDrawioImportRewordedNoteIsANote(t *testing.T) {
	golden, _, y := drawioNoteCase(t)
	for name, tc := range map[string]struct{ value, html, want string }{
		"single line": {"Checked weekly", "", "Checked weekly"},
		"multi line":  {"Checked weekly&#xa;by the operator", "", "Checked weekly\nby the operator"},
		"formatted":   {"Checked&amp;nbsp;weekly&lt;br&gt;by the &amp;lt;operator&amp;gt;", "1", "Checked weekly\nby the <operator>"},
	} {
		t.Run(name, func(t *testing.T) {
			document := setCellAttr(t, golden, "note0", "value", tc.value)
			if tc.html != "" {
				document = strings.Replace(document, `shape=note;html=0;`, `shape=note;html=`+tc.html+`;`, 1)
			}
			imp := drawioGoldenImport(t, layoutCase, document)
			edit := onlyEdit(t, imp, DrawioEditNote)
			if edit.Note == nil || edit.Note.Text != tc.want || edit.Note.Y != y || edit.Index != 0 || edit.Origin.Doc == "" {
				t.Errorf("note = %+v at %+v", edit.Note, edit.Origin)
			}
			if len(imp.Notices) != 0 {
				t.Errorf("notices = %q", imp.Notices)
			}
		})
	}
}

func TestDrawioImportRewordedAndMovedNoteIsOneNote(t *testing.T) {
	golden, _, _ := drawioNoteCase(t)
	document := mutateCell(t, setCellAttr(t, golden, "note0", "value", "Moved and reworded"), "note0", func(cell string) string {
		return setGeometryAttr(t, cell, "y", "333")
	})
	edit := onlyEdit(t, drawioGoldenImport(t, layoutCase, document), DrawioEditNote)
	if edit.Note == nil || edit.Note.Text != "Moved and reworded" || edit.Note.Y != 333 {
		t.Errorf("note = %+v", edit.Note)
	}
}

func TestDrawioImportReportsAnEmptiedNote(t *testing.T) {
	golden, text, _ := drawioNoteCase(t)
	for _, value := range []string{"", " &#xa; ", "&lt;br&gt;"} {
		document := setCellAttr(t, golden, "note0", "value", value)
		if value == "&lt;br&gt;" {
			document = strings.Replace(document, `shape=note;html=0;`, `shape=note;html=1;`, 1)
		}
		imp := drawioGoldenImport(t, layoutCase, document)
		want := "note note0 (" + strconv.Quote(firstLine(text)) + ") was emptied; a Note with no text is not a Note, so the note is left as it was"
		if len(imp.Edits) != 0 || len(imp.Notices) != 1 || imp.Notices[0] != want {
			t.Errorf("value %q: edits %+v notices %q, want %q", value, imp.Edits, imp.Notices, want)
		}
	}
}

// drawioPicturedRendering renders two pictures of local.png under dir, beside
// other.png, which the diagram may point a picture at instead.
func drawioPicturedRendering(t *testing.T) (*Rendering, string, string) {
	t.Helper()
	dir := t.TempDir()
	for name, side := range map[string]int{"local.png": 2, "other.png": 3} {
		var data bytes.Buffer
		if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, side, side))); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	rendering := &Rendering{View: "Views::pictured", Kind: KindTree, Roots: []*Node{{ID: "n", Kind: "part", Name: "part"}},
		Pictures: []Picture{
			{Location: "local.png", Dir: dir, X: 10, Y: 20, Width: 40, Height: 30, Alt: "the site"},
			{Location: "local.png", Dir: dir, X: 60, Y: 20, Width: 40, Height: 30, Above: true},
		}}
	exported, err := rendering.Drawio()
	if err != nil {
		t.Fatal(err)
	}
	return rendering, exported, dir
}

func drawioPicturedImport(t *testing.T, rendering *Rendering, document string) *DrawioImport {
	t.Helper()
	file, err := ReadDrawio([]byte(document), "")
	if err != nil {
		t.Fatal(err)
	}
	imp, err := rendering.ImportDrawio(file, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return imp
}

func TestDrawioImportChangedPictureSourceIsAPicture(t *testing.T) {
	rendering, exported, dir := drawioPicturedRendering(t)
	if imp := drawioPicturedImport(t, rendering, exported); len(imp.Edits) != 0 || len(imp.Notices) != 0 {
		t.Fatalf("untouched: edits %+v notices %q", imp.Edits, imp.Notices)
	}
	for name, tc := range map[string]struct{ src, location string }{
		"relative path": {"other.png", "other.png"},
		"absolute path": {filepath.Join(dir, "other.png"), "other.png"},
		"file URL":      {"file://" + filepath.ToSlash(filepath.Join(dir, "other.png")), "other.png"},
		"dotted path":   {"sub/../other.png", "other.png"},
	} {
		t.Run(name, func(t *testing.T) {
			imp := drawioPicturedImport(t, rendering, setImage(t, exported, "picture0", tc.src))
			edit := onlyEdit(t, imp, DrawioEditPicture)
			if edit.Picture == nil || edit.Picture.Location != tc.location || edit.Picture.X != 10 || edit.Picture.Alt != "the site" || edit.Index != 0 {
				t.Errorf("picture = %+v", edit.Picture)
			}
			if len(imp.Notices) != 0 {
				t.Errorf("notices = %q", imp.Notices)
			}
		})
	}
	t.Run("source and move", func(t *testing.T) {
		document := mutateCell(t, setImage(t, exported, "picture1", "other.png"), "picture1", func(cell string) string {
			return setGeometryAttr(t, cell, "x", "715")
		})
		edit := onlyEdit(t, drawioPicturedImport(t, rendering, document), DrawioEditPicture)
		if edit.Picture == nil || edit.Picture.Location != "other.png" || edit.Picture.X != 715 || !edit.Picture.Above {
			t.Errorf("picture = %+v", edit.Picture)
		}
	})
}

func TestDrawioImportReportsPictureSourcesItCannotName(t *testing.T) {
	rendering, exported, dir := drawioPicturedRendering(t)
	outside := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-outside.png")
	if err := os.WriteFile(outside, mustReadFile(t, filepath.Join(dir, "other.png")), 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outside)
	outsideURL := "file://" + filepath.ToSlash(outside)
	escape := "../" + filepath.Base(outside)
	for name, tc := range map[string]struct{ src, problem string }{
		"data URI":      {"data:image/png,iVBORw0KGgo=", "embedded image data, which no Picture location names (a Picture names a file)"},
		"remote URL":    {"https://example.org/bench.png", "https://example.org/bench.png, a URL no drawing tool reads"},
		"outside dir":   {outside, outside + ", a file outside the view's directory, which the import does not read"},
		"outside URL":   {outsideURL, outsideURL + ", a file outside the view's directory, which the import does not read"},
		"escaping path": {escape, escape + ", a file outside the view's directory, which the import does not read"},
		"system file":   {"/etc/hostname", "/etc/hostname, a file outside the view's directory, which the import does not read"},
		"missing":       {"missing.png", `"missing.png", a file that does not read`},
		"not image":     {"notes.txt", `"notes.txt", which is not an image`},
		"no image":      {"", "no image"},
	} {
		t.Run(name, func(t *testing.T) {
			imp := drawioPicturedImport(t, rendering, setImage(t, exported, "picture0", tc.src))
			want := "picture picture0 (local.png): its image was changed to " + tc.problem
			if len(imp.Edits) != 0 || len(imp.Notices) != 1 || !strings.HasPrefix(imp.Notices[0], want) || !strings.HasSuffix(imp.Notices[0], ", so the picture keeps its file") {
				t.Errorf("edits %+v notices %q, want %q", imp.Edits, imp.Notices, want)
			}
		})
	}
	// Moved as well: the move imports with the picture's own file, the source is reported.
	document := mutateCell(t, setImage(t, exported, "picture0", "data:image/png,iVBORw0KGgo="), "picture0", func(cell string) string {
		return setGeometryAttr(t, cell, "x", "15")
	})
	imp := drawioPicturedImport(t, rendering, document)
	edit := onlyEdit(t, imp, DrawioEditPicture)
	if edit.Picture == nil || edit.Picture.Location != "local.png" || edit.Picture.X != 15 || len(imp.Notices) != 1 {
		t.Errorf("picture = %+v notices %q", edit.Picture, imp.Notices)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
