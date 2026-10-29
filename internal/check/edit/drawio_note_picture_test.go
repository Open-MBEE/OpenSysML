package edit

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

const drawioAnnotated = `package Plant {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	part def Pump;
	part def Loop {
		part pump : Pump;
	}
	view loopView {
		expose Loop::*;
		render asInterconnectionDiagram;
		metadata Layout about Loop::pump { x = 100; y = 100; width = 120; height = 60; }
		metadata Note about Loop::pump { text = "runs"; x = 300; y = 100; width = 100; height = 40; }
		@Picture { location = "local.png"; x = 0; y = 0; width = 40; height = 30; alt = "the site"; }
	}
}
`

// drawioAnnotatedModel loads drawioAnnotated from a directory holding local.png
// and other.png, so its Picture reads and a diagram may point at the other file.
func drawioAnnotatedModel(t *testing.T) (Model, *view.Rendering, view.Options, string) {
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
	m, rendering, options := drawioModelOf(t, loadContent(t, filepath.Join(dir, "model.sysml"), drawioAnnotated), "Plant::loopView")
	return m, rendering, options, dir
}

func TestDrawioImportAppliesANoteText(t *testing.T) {
	m, rendering, options, dir := drawioAnnotatedModel(t)
	document, _ := drawioExport(t, rendering, options)
	for _, tc := range []struct{ value, text, literal string }{
		{"Checked weekly", "Checked weekly", `"Checked weekly"`},
		{"Checked weekly&#xa;by the &quot;operator&quot;", "Checked weekly\nby the \"operator\"", `"Checked weekly\nby the \"operator\""`},
	} {
		edited := drawioMutate(t, document, "note0", func(cell string) string { return drawioSetValue(cell, tc.value) })
		_, imp := drawioImport(t, drawioImporter(t, rendering, options), rendering.View, edited)
		if len(imp.Edits) != 1 || imp.Edits[0].Kind != view.DrawioEditNote || imp.Edits[0].Note.Text != tc.text || imp.Edits[0].Note.X != 300 {
			t.Fatalf("edits = %+v", imp.Edits)
		}
		res, err := Apply(m, DrawioOperations(imp, false))
		if err != nil {
			t.Fatal(err)
		}
		want := `metadata Note about Loop::pump { text = ` + tc.literal + `; x = 300; y = 100; width = 100; height = 40; }`
		if !strings.Contains(string(res.Content), want) {
			t.Errorf("applied:\n%s\nwant %s", res.Content, want)
		}
		_, redrawn, reoptions := drawioModelOf(t, loadContent(t, filepath.Join(dir, "model.sysml"), string(res.Content)), "Plant::loopView")
		_, re := drawioExport(t, redrawn, reoptions)
		if note := re.Cell("note0"); note == nil || note.Value != tc.text {
			t.Errorf("re-exported note = %+v, want text %q", note, tc.text)
		}
		if _, again := drawioImport(t, drawioImporter(t, redrawn, reoptions), redrawn.View, edited); len(again.Edits) != 0 || len(again.Notices) != 0 {
			t.Errorf("the edited file still imports %+v %q", again.Edits, again.Notices)
		}
	}
}

func TestDrawioImportAppliesAPictureSource(t *testing.T) {
	m, rendering, options, dir := drawioAnnotatedModel(t)
	document, exported := drawioExport(t, rendering, options)
	if exported.Cell("picture0") == nil {
		t.Fatalf("no picture drawn:\n%s", document)
	}
	other, err := os.ReadFile(filepath.Join(dir, "other.png"))
	if err != nil {
		t.Fatal(err)
	}
	edited := drawioMutate(t, document, "picture0", func(cell string) string {
		return regexp.MustCompile(`image=[^;"]*;`).ReplaceAllLiteralString(cell, "image=file://"+filepath.ToSlash(filepath.Join(dir, "other.png"))+";")
	})
	_, imp := drawioImport(t, drawioImporter(t, rendering, options), rendering.View, edited)
	if len(imp.Edits) != 1 || imp.Edits[0].Kind != view.DrawioEditPicture || imp.Edits[0].Picture.Location != "other.png" || imp.Edits[0].Picture.Alt != "the site" {
		t.Fatalf("edits = %+v notices %q", imp.Edits, imp.Notices)
	}
	res, err := Apply(m, DrawioOperations(imp, false))
	if err != nil {
		t.Fatal(err)
	}
	want := `@Picture { location = "other.png"; x = 0; y = 0; width = 40; height = 30; alt = "the site"; }`
	if !strings.Contains(string(res.Content), want) {
		t.Errorf("applied:\n%s\nwant %s", res.Content, want)
	}
	_, redrawn, reoptions := drawioModelOf(t, loadContent(t, filepath.Join(dir, "model.sysml"), string(res.Content)), "Plant::loopView")
	_, re := drawioExport(t, redrawn, reoptions)
	if picture := re.Cell("picture0"); picture == nil || picture.Style["image"] != "data:image/png,"+base64.StdEncoding.EncodeToString(other) {
		t.Errorf("re-exported picture = %+v, want the other file's data", picture)
	}

	// Embedded data is reported and the picture keeps its file.
	edited = drawioMutate(t, document, "picture0", func(cell string) string {
		return regexp.MustCompile(`image=[^;"]*;`).ReplaceAllLiteralString(cell, "image=data:image/png,"+base64.StdEncoding.EncodeToString(other)+";")
	})
	if _, imp := drawioImport(t, drawioImporter(t, rendering, options), rendering.View, edited); len(imp.Edits) != 0 || len(imp.Notices) != 1 || !strings.Contains(imp.Notices[0], "embedded image data") {
		t.Errorf("edits %+v notices %q", imp.Edits, imp.Notices)
	}
}
