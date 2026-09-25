package docrender

import (
	"path/filepath"
	"strings"
	"testing"
)

// An image block's relative location resolves against the directory of the source file
// stating it; an absolute path or file URL as written; a remote location not at all.
func TestImagesResolveBesideTheSource(t *testing.T) {
	model := filepath.Join(t.TempDir(), "model")
	document := fixtureDocumentAt(t, filepath.Join("testdata", "image_report.sysml"), filepath.Join(model, "report.sysml"), "Pictures::ImageReport")
	images := Images(document)
	if len(images) != 4 {
		t.Fatalf("Images = %d blocks, want 4", len(images))
	}
	for _, image := range images {
		if image.Dir != model {
			t.Errorf("%s: Dir = %q, want the model's directory %q", image.Name, image.Dir, model)
		}
	}
	base := t.TempDir()
	tests := []struct {
		name     string
		relative bool
		remote   bool
		path     string
	}{
		{"plate", true, false, filepath.Join(model, "images", "mark.png")},
		{"remote", false, true, ""},
		{"spaced", true, false, filepath.Join(model, "images", "plate 3.png")},
		{"tricky", true, false, filepath.Join(model, "images", "a) <b>.png")},
	}
	for i, tt := range tests {
		image := images[i]
		if image.Name != tt.name {
			t.Fatalf("Images[%d] = %s, want %s", i, image.Name, tt.name)
		}
		if image.Relative() != tt.relative || image.Remote() != tt.remote {
			t.Errorf("%s: Relative = %t, Remote = %t, want %t and %t", tt.name, image.Relative(), image.Remote(), tt.relative, tt.remote)
		}
		if got := image.Path(base); got != tt.path {
			t.Errorf("%s: Path = %q, want %q", tt.name, got, tt.path)
		}
	}
	absolute := Image{Location: "/srv/plates/mark.png", Dir: model}
	if got := absolute.Path(base); got != "/srv/plates/mark.png" {
		t.Errorf("absolute Path = %q, want the path as written", got)
	}
	fileURL := Image{Location: "file:///srv/plates/mark.png", Dir: model}
	if got := fileURL.Path(base); got != filepath.FromSlash("/srv/plates/mark.png") {
		t.Errorf("file URL Path = %q, want the URL's path", got)
	}
	sourceless := Image{Location: "images/mark.png"}
	if got := sourceless.Path(base); got != filepath.Join(base, "images", "mark.png") {
		t.Errorf("sourceless Path = %q, want the location under base", got)
	}
}

// Written into another directory, a rendering refers to the file by a path from there; an
// unknown output directory, a sourceless document or a non-relative location keep it as stated.
func TestImageSourceIsRelativeToTheOutput(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, "model")
	image := Image{Location: "images/mark.png", Dir: model}
	tests := []struct {
		image  Image
		output string
		want   string
	}{
		{image, filepath.Join(root, "out", "html"), "../../model/images/mark.png"},
		{image, model, "images/mark.png"},
		{image, "", "images/mark.png"},
		{Image{Location: "images/mark.png"}, filepath.Join(root, "out"), "images/mark.png"},
		{Image{Location: "https://example.test/mark.png", Dir: model}, filepath.Join(root, "out"), "https://example.test/mark.png"},
		{Image{Location: "/srv/mark.png", Dir: model}, filepath.Join(root, "out"), "/srv/mark.png"},
	}
	for _, tt := range tests {
		if got := tt.image.Source(tt.output); got != tt.want {
			t.Errorf("Source(%q) of %+v = %q, want %q", tt.output, tt.image, got, tt.want)
		}
	}
}

// The HTML and Markdown backends agree on the reference a page in another directory makes
// to an image beside the source, and write the location as stated without an output directory.
func TestBackendsWriteImagesRelativeToTheOutput(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, "model")
	out := filepath.Join(root, "out")
	document := fixtureDocumentAt(t, filepath.Join("testdata", "image_report.sysml"), filepath.Join(model, "report.sysml"), "Pictures::ImageReport")
	page, err := HTML(document, HTMLOptions{Fragment: true, OutputDir: out})
	if err != nil {
		t.Fatal(err)
	}
	markdown, err := Markdown(document, MarkdownOptions{OutputDir: out})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<img src="../model/images/mark.png"`, `<img src="https://example.test/plates/plate2.png"`, `<img src="../model/images/plate 3.png"`} {
		if !strings.Contains(page, want) {
			t.Errorf("HTML lacks %s:\n%s", want, page)
		}
	}
	for _, want := range []string{"![a brass survey mark](../model/images/mark.png)", "![](<../model/images/plate 3.png>)", "![odd name](<../model/images/a%29 %3Cb%3E.png>)"} {
		if !strings.Contains(markdown, want) {
			t.Errorf("Markdown lacks %s:\n%s", want, markdown)
		}
	}
	page, err = HTML(document, HTMLOptions{Fragment: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, `<img src="images/mark.png"`) {
		t.Errorf("HTML without an output directory does not keep the location as stated:\n%s", page)
	}
}
