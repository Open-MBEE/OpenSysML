package view

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A view's Pictures reach the rendering with their bounds, and the DOT form
// pins each as an image node: those under the parts first, those above last.
func TestPicturesReachTheRenderingAndTheDOTForm(t *testing.T) {
	rendering := render(t, "pictures.sysml", "Site::mixedView")
	want := []Picture{
		{Location: "images/bench.png", Dir: ".", X: 0, Y: 0, Width: 823, Height: 577, Alt: "the bench, from above"},
		{Location: "images/logo.png", Dir: ".", X: 700, Y: 20, Width: 80, Height: 40, Above: true},
	}
	if !reflect.DeepEqual(rendering.Pictures, want) {
		t.Errorf("pictures = %+v, want %+v", rendering.Pictures, want)
	}
	if rendering.Empty() || !rendering.Positioned() {
		t.Errorf("a view of placed parts and pictures is empty=%v positioned=%v", rendering.Empty(), rendering.Positioned())
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	// The canvas is 600 high: the bench's centre (411.5, 288.5) flips to y = 311.5.
	background := `"picture:0" [shape=none, style="", label="", image="images/bench.png", imagescale=both, fixedsize=true, width=11.430555555555555, height=8.01388888888889, pos="411.5,311.5!", pin=true, tooltip="the bench, from above"];`
	overlay := `"picture:1" [shape=none, style="", label="", image="images/logo.png", imagescale=both, fixedsize=true, width=1.1111111111111112, height=0.5555555555555556, pos="740,560!", pin=true];`
	for _, want := range []string{background, overlay} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	bench := strings.Index(dot, `label=<`)
	if bg, ov := strings.Index(dot, background), strings.Index(dot, overlay); !(bg < bench && bench < ov) {
		t.Errorf("the background picture is not written before the parts and the overlay after them:\n%s", dot)
	}
	if !strings.Contains(dot, "neato -n") {
		t.Errorf("the pictured view is not positioned:\n%s", dot)
	}
}

// Pictures under and over the parts interleaved in declaration order are written
// as two layers, each in declaration order: under the parts, a later picture lies
// over an earlier one it overlaps, and so over the parts.
func TestPictureLayersKeepDeclarationOrder(t *testing.T) {
	rendering := render(t, "pictures.sysml", "Site::layeredView")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	at := func(id string) int {
		i := strings.Index(dot, `"`+id+`" [shape=none`)
		if i < 0 {
			t.Fatalf("DOT lacks %s:\n%s", id, dot)
		}
		return i
	}
	under0, over1, under2, over3, parts := at("picture:0"), at("picture:1"), at("picture:2"), at("picture:3"), strings.Index(dot, `label=<`)
	if !(under0 < under2 && under2 < parts && parts < over1 && over1 < over3) {
		t.Errorf("pictures are not written under the parts then over them, each layer in declaration order:\n%s", dot)
	}
}

// A picture places no node: a view whose nodes nothing positions draws them
// all, in a strip below the picture in DOT, whole in the other forms.
func TestPictureAloneLeavesNoNodeUndrawn(t *testing.T) {
	rendering := render(t, "pictures.sysml", "Site::unplacedView")
	if !rendering.Positioned() {
		t.Fatal("a pictured view is not positioned")
	}
	for _, options := range []Options{{}, {Unplaced: UnplacedStrip}} {
		dot, err := rendering.DOTWith(options)
		if err != nil {
			t.Fatalf("DOT %+v: %v", options, err)
		}
		checkDOTSyntax(t, dot)
		if !strings.Contains(dot, "// not represented: 3 node(s) without a position, drawn in a strip below the picture(s)\n") || strings.Contains(dot, "left undrawn") {
			t.Errorf("DOT %+v header:\n%s", options, dot)
		}
		// The table's cluster heads the strip 24 below the picture's 300 high box.
		if !strings.Contains(dot, `"picture:0" [shape=none`) || !strings.Contains(dot, `bb="0,-406,331,-324";`) || !strings.Contains(dot, "bench : Bench") || !strings.Contains(dot, "camera : Camera") {
			t.Errorf("DOT %+v leaves the parts undrawn:\n%s", options, dot)
		}
	}
	if mermaid := rendering.Mermaid(); !strings.Contains(mermaid, "bench") || !strings.Contains(mermaid, "camera") || strings.Contains(mermaid, "without a position") {
		t.Errorf("Mermaid leaves the parts undrawn:\n%s", mermaid)
	}
}

// A Picture stated in another file, `about` the view, locates its file from
// that file's directory, not the view's.
func TestPictureStatedElsewhereIsLocatedFromItsOwnFile(t *testing.T) {
	site := []byte(`package Site {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;
	part def Bench;
	view benchView {
		expose Site::Bench;
		render asInterconnectionDiagram;
		@Picture { location = "images/bench.png"; x = 0; y = 0; width = 400; height = 300; }
	}
}
`)
	overlay := []byte(`package Overlay {
	private import DiagramLayout::*;
	metadata Picture about Site::benchView { location = "images/logo.png"; x = 10; y = 10; width = 80; height = 40; above = true; }
}
`)
	r, idx := loadSources(t, []string{filepath.Join("model", "site.sysml"), filepath.Join("docs", "overlay", "overlay.sysml")}, [][]byte{site, overlay})
	rendering, err := r.Render(lookup(t, idx, "Site::benchView"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Picture{
		{Location: "images/bench.png", Dir: "model", X: 0, Y: 0, Width: 400, Height: 300},
		{Location: "images/logo.png", Dir: filepath.Join("docs", "overlay"), X: 10, Y: 10, Width: 80, Height: 40, Above: true},
	}
	if !reflect.DeepEqual(rendering.Pictures, want) {
		t.Errorf("pictures = %+v, want %+v", rendering.Pictures, want)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join("model", "images", "bench.png"), filepath.Join("docs", "overlay", "images", "logo.png")} {
		if !strings.Contains(dot, "image="+dotQuote(path)) {
			t.Errorf("DOT lacks image=%q:\n%s", path, dot)
		}
	}
}

// A view exposing nothing but carrying a picture is not empty: DOT draws the
// picture alone; the forms that draw no picture say so instead of a blank.
func TestPictureOnlyViewIsNotEmpty(t *testing.T) {
	rendering := render(t, "pictures.sysml", "Site::pictureOnlyView")
	if rendering.Empty() || !rendering.Positioned() {
		t.Fatalf("a view of a picture alone is empty=%v positioned=%v", rendering.Empty(), rendering.Positioned())
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	// No canvas: y is negated, as for every pinned node, and Graphviz translates.
	if want := `"picture:0" [shape=none, style="", label="", image="images/site.jpg", imagescale=both, fixedsize=true, width=8.88888888888889, height=6.666666666666667, pos="320,-240!", pin=true];`; !strings.Contains(dot, want) {
		t.Errorf("DOT lacks %q:\n%s", want, dot)
	}
	if strings.Contains(dot, "exposes nothing") {
		t.Errorf("DOT calls the pictured view empty:\n%s", dot)
	}
	notice := "not represented: 1 picture(s) not drawn: images/site.jpg at (0, 0) size 640×480; the dot form draws pictures"
	reason := "the view shows 1 picture(s), which the mermaid form does not draw"
	if mermaid := rendering.Mermaid(); !strings.Contains(mermaid, "%% "+notice) || !strings.Contains(mermaid, reason) {
		t.Errorf("Mermaid drops the picture silently:\n%s", mermaid)
	}
	puml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	if !strings.Contains(puml, "' "+notice) || !strings.Contains(puml, "the view shows 1 picture(s), which the plantuml form does not draw") {
		t.Errorf("PlantUML drops the picture silently:\n%s", puml)
	}
	if text := rendering.Text(); !strings.Contains(text, "pictures:\n  \"images/site.jpg\" at (0, 0) size 640×480\n") {
		t.Errorf("text lacks the picture:\n%s", text)
	}
	clone := rendering.Clone()
	clone.Pictures[0].Location = "elsewhere.png"
	if rendering.Pictures[0].Location != "images/site.jpg" {
		t.Error("Clone shares the pictures with the original")
	}
}

// A table draws no picture: the one its view states is noticed in every form,
// not carried as drawn and not dropped.
func TestPictureOnTableIsNoticedNotDrawn(t *testing.T) {
	rendering := render(t, "pictures.sysml", "Site::tableView")
	if len(rendering.Pictures) != 0 || len(rendering.Rows) != 2 {
		t.Fatalf("table carries %d picture(s) and %d row(s), want 0 and 2", len(rendering.Pictures), len(rendering.Rows))
	}
	notice := "1 picture(s) not drawn: images/site.jpg at (0, 0) size 640×480; a table rendering draws no picture"
	if len(rendering.Notices) != 1 || rendering.Notices[0] != notice {
		t.Fatalf("notices = %q, want [%q]", rendering.Notices, notice)
	}
	if text := rendering.Text(); !strings.Contains(text, notice) {
		t.Errorf("text drops the picture silently:\n%s", text)
	}
	if md := rendering.Markdown(); !strings.Contains(md, notice) {
		t.Errorf("markdown drops the picture silently:\n%s", md)
	}
}

// A picture's path is its location resolved against the document's directory,
// with absolute paths and URLs left as they are.
func TestPicturePath(t *testing.T) {
	abs := filepath.Join(string(filepath.Separator), "srv", "model", "images", "a.png")
	cases := []struct {
		picture Picture
		want    string
	}{
		{Picture{Location: "images/a.png", Dir: filepath.Join("srv", "model")}, filepath.Join("srv", "model", "images", "a.png")},
		{Picture{Location: "images/a.png"}, "images/a.png"},
		{Picture{Location: abs, Dir: "elsewhere"}, abs},
		{Picture{Location: "https://example.org/a.png", Dir: "elsewhere"}, "https://example.org/a.png"},
	}
	for _, c := range cases {
		if got := c.picture.Path(); got != c.want {
			t.Errorf("%+v.Path() = %q, want %q", c.picture, got, c.want)
		}
	}
}
