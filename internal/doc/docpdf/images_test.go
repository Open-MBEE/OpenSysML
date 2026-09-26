package docpdf

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
)

// TestRenderImageMissing reports a clear error naming the block when the file
// a local image location resolves to, beside the document's source, does not
// exist.
func TestRenderImageMissing(t *testing.T) {
	dir := t.TempDir()
	fakeTool(t, dir, "weasyprint", WeasyPrintEnv, "exit 0\n")
	model := t.TempDir()
	document := imageDocSource(t, filepath.Join(model, "image.sysml"), `"images/mark.png"`)
	_, err := Render(document, "weasyprint", Options{BaseDir: t.TempDir()})
	var render *Error
	if !errors.As(err, &render) || render.Kind != ErrorImageMissing {
		t.Fatalf("Render error = %v, want missing-image", err)
	}
	if render.Detail != filepath.Join(model, "images", "mark.png") {
		t.Errorf("error detail = %q, want the file beside the source", render.Detail)
	}
}

// TestRenderRemoteImageSkipsTheCheck leaves an http(s) location for the
// engine to fetch: no local file is required.
func TestRenderRemoteImageSkipsTheCheck(t *testing.T) {
	dir := t.TempDir()
	fakeMermaid(t, dir)
	capture := captureWeasyPrint(t, dir)
	document := imageDocSource(t, "image.sysml", `"https://example.test/plate.png"`)
	if _, err := Render(document, "weasyprint", Options{BaseDir: dir}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, _ := readCapture(t, capture)
	if !strings.Contains(page, `<img src="https://example.test/plate.png"`) {
		t.Errorf("remote image not in page:\n%s", page)
	}
}

// TestRenderLocalImageResolvesBesideTheSource checks a relative location resolves
// against the source file and is written relative to the PDF's directory.
func TestRenderLocalImageResolvesBesideTheSource(t *testing.T) {
	dir := t.TempDir()
	fakeMermaid(t, dir)
	capture := captureWeasyPrint(t, dir)
	root := t.TempDir()
	model := filepath.Join(root, "model")
	out := filepath.Join(root, "out", "pdf")
	writeMark(t, filepath.Join(model, "images", "mark.png"))
	if err := os.MkdirAll(out, 0o750); err != nil {
		t.Fatal(err)
	}
	document := imageDocSource(t, filepath.Join(model, "image.sysml"), `"images/mark.png"`)
	if _, err := Render(document, "weasyprint", Options{BaseDir: out}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, _ := readCapture(t, capture)
	if !strings.Contains(page, `<img src="../../model/images/mark.png"`) {
		t.Errorf("image not referenced relative to the PDF's directory:\n%s", page)
	}
}

// TestRenderLocalImageOfSourcelessModelResolvesAgainstBase checks a document
// read from no file on disk resolves its images against the PDF's directory.
func TestRenderLocalImageOfSourcelessModelResolvesAgainstBase(t *testing.T) {
	dir := t.TempDir()
	fakeMermaid(t, dir)
	capture := captureWeasyPrint(t, dir)
	base := t.TempDir()
	writeMark(t, filepath.Join(base, "images", "mark.png"))
	document := imageDocSource(t, "<stdin>", `"images/mark.png"`)
	if _, err := Render(document, "weasyprint", Options{BaseDir: base}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	page, _ := readCapture(t, capture)
	if !strings.Contains(page, `<img src="images/mark.png"`) {
		t.Errorf("relative image not in page:\n%s", page)
	}
}

// TestRenderImageBesideTheSourceWithInstalledEngines checks each converter draws
// the image beside a model whose directory is not the PDF's.
func TestRenderImageBesideTheSourceWithInstalledEngines(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, "model")
	out := filepath.Join(root, "out")
	writeMark(t, filepath.Join(model, "images", "mark.png"))
	if err := os.MkdirAll(out, 0o750); err != nil {
		t.Fatal(err)
	}
	document := imageDocSource(t, filepath.Join(model, "image.sysml"), `"images/mark.png"`)
	for _, engine := range Engines() {
		t.Run(engine, func(t *testing.T) {
			pdf, text := renderInstalled(t, document, engine, Options{BaseDir: out})
			if !strings.Contains(text, "The survey mark") {
				t.Errorf("the caption is not in the PDF:\n%s", text)
			}
			images := pdfImages(t, pdf)
			if !regexp.MustCompile(`(?m)^\s*1\s+0\s+image\s+12\s+12\s`).MatchString(images) {
				t.Errorf("the image beside the model was not drawn:\n%s", images)
			}
		})
	}
}

// writeMark copies the 12x12 fixture image to path, creating its directory.
func writeMark(t *testing.T, path string) {
	t.Helper()
	mark, err := os.ReadFile(filepath.Join("testdata", "mark.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, mark, 0o600); err != nil {
		t.Fatal(err)
	}
}

// imageDocSource evaluates a one-image report from a source of the given
// file name with the given location literal.
func imageDocSource(t *testing.T, file, location string) *docir.Document {
	t.Helper()
	return sourceDocument(t, file, `package Pictures {
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
