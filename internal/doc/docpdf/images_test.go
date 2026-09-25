package docpdf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
)

// TestRenderImageMissing reports a clear error naming the block when the file
// a local image location resolves to does not exist.
func TestRenderImageMissing(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	document := imageDocSource(t, `"images/mark.png"`)
	_, err := Render(document, "weasyprint", Options{BaseDir: t.TempDir()})
	var render *Error
	if !errors.As(err, &render) || render.Kind != ErrorImageMissing {
		t.Fatalf("Render error = %v, want missing-image", err)
	}
	if !strings.Contains(render.Detail, filepath.Join("images", "mark.png")) {
		t.Errorf("error detail = %q", render.Detail)
	}
}

// TestRenderRemoteImageSkipsTheCheck leaves an http(s) location for the
// engine to fetch: no local file is required.
func TestRenderRemoteImageSkipsTheCheck(t *testing.T) {
	dir := t.TempDir()
	fakeMermaid(t, dir)
	capture := captureWeasyPrint(t, dir)
	document := imageDocSource(t, `"https://example.test/plate.png"`)
	if _, err := Render(document, "weasyprint", Options{BaseDir: dir}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, _ := readCapture(t, capture)
	if !strings.Contains(page, `<img src="https://example.test/plate.png"`) {
		t.Errorf("remote image not in page:\n%s", page)
	}
}

// TestRenderLocalImageWritesRelativeReference checks the page references the
// image exactly as the document's location states, relative to the base.
func TestRenderLocalImageWritesRelativeReference(t *testing.T) {
	dir := t.TempDir()
	fakeMermaid(t, dir)
	capture := captureWeasyPrint(t, dir)
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "images"), 0o750); err != nil {
		t.Fatal(err)
	}
	mark, err := os.ReadFile(filepath.Join("testdata", "mark.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "images", "mark.png"), mark, 0o600); err != nil {
		t.Fatal(err)
	}
	document := imageDocSource(t, `"images/mark.png"`)
	if _, err := Render(document, "weasyprint", Options{BaseDir: base}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, _ := readCapture(t, capture)
	if !strings.Contains(page, `<img src="images/mark.png"`) {
		t.Errorf("relative image not in page:\n%s", page)
	}
}

// imageDocSource evaluates a one-image report with the given location literal.
func imageDocSource(t *testing.T, location string) *docir.Document {
	t.Helper()
	return sourceDocument(t, "image.sysml", `package Pictures {
	private import DocumentQueries::*;
	part def Report :> Document {
		attribute redefines title = "Plates";
		part photo : Image {
			attribute redefines location = `+location+`;
			attribute redefines caption = "The survey mark";
			attribute redefines alt = "a mark";
		}
	}
}
`, "Pictures::Report")
}
