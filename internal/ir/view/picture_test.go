package view

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/imagefile"
)

func TestRemotePictureLocation(t *testing.T) {
	for _, location := range []string{
		"http://x", "https://x", "file:///x", "ftp://x", "scheme://x",
		"javascript:alert(1)", "FILE:/x", "mailto:a@b",
	} {
		if !RemotePictureLocation(location) {
			t.Errorf("RemotePictureLocation(%q) = false, want true", location)
		}
	}
	for _, location := range []string{
		"images/a.png", "/abs/a.png", `C:\x.png`, "C:/x.png",
		"data:image/png;base64,AAAA", "a.png",
	} {
		if RemotePictureLocation(location) {
			t.Errorf("RemotePictureLocation(%q) = true, want false", location)
		}
	}
}

func TestCheckPicture(t *testing.T) {
	script := `<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`
	scriptData := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(script))
	encoded := "data:image/svg+xml," + url.PathEscape(script)
	for _, location := range []string{scriptData, encoded} {
		err := CheckPicture(location, nil)
		var active *imagefile.ActiveContentError
		if !errors.As(err, &active) || active.Construct != "<script>" {
			t.Errorf("CheckPicture(%q) = %v, want active-content error", location, err)
		}
	}
	for location, want := range map[string]string{
		"data:text/plain,hi":        "the data: URL is not a supported image",
		"data:image/png;base64,%%%": "the data: URL does not decode",
		"script.svg":                "the SVG is not well-formed",
	} {
		var data []byte
		if location == "script.svg" {
			data = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
		}
		err := CheckPicture(location, data)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("CheckPicture(%q) = %v, want an error containing %q", location, err, want)
		}
	}
	clean := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect fill="url(#g)"/></svg>`)
	if err := CheckPicture("clean.svg", clean); err != nil {
		t.Errorf("clean SVG: %v", err)
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	if err := CheckPicture("clean.png", pngData.Bytes()); err != nil {
		t.Errorf("clean PNG: %v", err)
	}
}

func hardeningPictureRendering(t *testing.T) (*Rendering, []string, []byte) {
	t.Helper()
	dir := t.TempDir()
	locations := []string{
		filepath.Join(dir, "script.svg"),
		"https://example.org/a.png",
		filepath.Join(dir, "clean.svg"),
		filepath.Join(dir, "a.png"),
	}
	if err := os.WriteFile(locations[0], []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(locations[2], []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`), 0o600); err != nil {
		t.Fatal(err)
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(locations[3], pngData.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	pictures := make([]Picture, len(locations))
	for i, location := range locations {
		pictures[i] = Picture{Location: location, X: 1, Y: 2, Width: 30, Height: 40}
	}
	return &Rendering{
		View:     "Pictures",
		Kind:     KindInterconnection,
		Roots:    []*Node{{ID: "n", Kind: "part", Name: "part"}},
		Pictures: pictures,
	}, locations, pngData.Bytes()
}

func TestUnsafePicturesAreOmittedInEveryViewForm(t *testing.T) {
	rendering, locations, _ := hardeningPictureRendering(t)
	activeNotice := pictureNotice([]Picture{rendering.Pictures[0]}, "the SVG has active content (<script>)")
	remoteNotice := pictureNotice([]Picture{rendering.Pictures[1]}, ErrRemotePicture.Error())

	mermaid := rendering.Mermaid()
	for _, want := range []string{
		"%% not represented: " + activeNotice,
		"%% not represented: " + remoteNotice,
		"picture2@{ img: \"" + locations[2] + "\"",
		"picture3@{ img: \"" + locations[3] + "\"",
	} {
		if !strings.Contains(mermaid, want) {
			t.Errorf("Mermaid lacks %q:\n%s", want, mermaid)
		}
	}
	for _, omitted := range []string{"picture0@{", "picture1@{", "%% layout: picture0 ", "%% layout: picture1 "} {
		if strings.Contains(mermaid, omitted) {
			t.Errorf("Mermaid includes refused picture marker %q:\n%s", omitted, mermaid)
		}
	}
	inlined := InlineMermaidImages(mermaid, filepath.Dir(locations[0]))
	if !strings.Contains(inlined, "data:image/svg+xml;base64,") || !strings.Contains(inlined, "data:image/png;base64,") {
		t.Errorf("Mermaid did not inline the safe SVG and PNG:\n%s", inlined)
	}

	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{
		"// not represented: " + activeNotice,
		"// not represented: " + remoteNotice,
		`"picture:2" [shape=none`,
		`"picture:3" [shape=none`,
		"image=" + dotQuote(locations[2]),
		"image=" + dotQuote(locations[3]),
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	for _, omitted := range []string{`"picture:0" [shape=none`, `"picture:1" [shape=none`, "image=" + dotQuote(locations[0]), `image="https://example.org/a.png"`} {
		if strings.Contains(dot, omitted) {
			t.Errorf("DOT includes refused picture marker %q:\n%s", omitted, dot)
		}
	}

	plantuml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	d2, err := rendering.D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	drawableNotice := pictureNotice(rendering.Pictures[2:], "the dot form draws pictures")
	for form, source := range map[string]string{"PlantUML": plantuml, "D2": d2} {
		prefix := "'"
		if form == "D2" {
			prefix = "#"
		}
		for _, notice := range []string{activeNotice, remoteNotice, drawableNotice} {
			if !strings.Contains(source, prefix+" not represented: "+notice) {
				t.Errorf("%s lacks notice %q:\n%s", form, notice, source)
			}
		}
	}
	if got := rendering.DataFor(PortsMinimal).Notices; len(got) != 2 || got[0] != activeNotice || got[1] != remoteNotice {
		t.Errorf("DataFor notices = %q, want [%q %q]", got, activeNotice, remoteNotice)
	}
}

func TestInlineMermaidImagesDropsUnsafePictures(t *testing.T) {
	_, locations, pngData := hardeningPictureRendering(t)
	activeData := "data:image/svg+xml," + url.PathEscape(`<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`)
	pngDataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData)
	source := strings.Join([]string{
		`flowchart LR`,
		fmt.Sprintf(`  picture0@{ img: "%s", label: "", w: 1, h: 1 }`, locations[0]),
		`  picture1@{ img: "https://example.org/a.png", label: "", w: 1, h: 1 }`,
		fmt.Sprintf(`  picture2@{ img: "%s", label: "", w: 1, h: 1 }`, activeData),
		fmt.Sprintf(`  picture3@{ img: "%s", label: "", w: 1, h: 1 }`, pngDataURL),
	}, "\n")
	got := InlineMermaidImages(source, filepath.Dir(locations[0]))
	for _, want := range []string{
		"%% not represented: picture " + locations[0] + " not drawn; the SVG has active content (<script>)",
		"%% not represented: picture https://example.org/a.png not drawn; remote pictures are not drawn",
		"%% not represented: picture " + activeData + " not drawn; the SVG has active content (<script>)",
		`picture3@{ img: "` + pngDataURL + `"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("inlined Mermaid lacks %q:\n%s", want, got)
		}
	}
	for _, omitted := range []string{"picture0@{", "picture1@{", "picture2@{", `img: "` + locations[0] + `"`} {
		if strings.Contains(got, omitted) {
			t.Errorf("inlined Mermaid contains refused content %q:\n%s", omitted, got)
		}
	}
	if count := strings.Count(got, "<script"); count != 2 {
		t.Errorf("inlined Mermaid has %d script construct(s), want only the notices:\n%s", count, got)
	}
}

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
	mermaidNotice := "not represented: 1 picture(s) not drawn: images/site.jpg at (0, 0) size 640×480; the file does not read"
	if mermaid := rendering.Mermaid(); !strings.Contains(mermaid, "%% "+mermaidNotice) || strings.Contains(mermaid, "picture0@{") {
		t.Errorf("Mermaid drops the picture silently:\n%s", mermaid)
	}
	puml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	if !strings.Contains(puml, "' not represented: 1 picture(s) not drawn: images/site.jpg at (0, 0) size 640×480; the dot form draws pictures") ||
		!strings.Contains(puml, "the view shows 1 picture(s), which the plantuml form does not draw") {
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

// The layers hold in what Graphviz paints, not only in the DOT text: the overlay
// covers the connections as well as the parts, the background lies under both.
func TestPictureLayersHoldInGraphvizOutput(t *testing.T) {
	dot, err := exec.LookPath("dot")
	if err != nil {
		t.Skip("dot is not installed")
	}
	rendering := render(t, "pictures.sysml", "Site::wiredView")
	if len(rendering.Edges) != 1 || len(rendering.Pictures) != 2 {
		t.Fatalf("view carries %d edge(s) and %d picture(s), want 1 and 2", len(rendering.Edges), len(rendering.Pictures))
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bench.png", "logo.png"} {
		var pic bytes.Buffer
		if err := png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "images", name), pic.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, style := range []DrawingStyle{StylePilot, StyleCameo} {
		source, err := rendering.DOTWith(Options{Style: style})
		if err != nil {
			t.Fatalf("%s: DOTWith: %v", style, err)
		}
		cmd := exec.Command(dot, "-Kneato", "-n", "-Tsvg")
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(source)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil || stderr.Len() != 0 {
			t.Fatalf("%s: dot -n: %v\nstderr: %s\nsource:\n%s", style, err, stderr.String(), source)
		}
		svg := string(out)
		at := func(title string) int {
			i := strings.Index(svg, "<title>"+title+"</title>")
			if i < 0 {
				t.Fatalf("%s: SVG draws no %q:\n%s", style, title, svg)
			}
			return i
		}
		under, bench, camera, wire, over := at("picture:0"), at("n1"), at("n2"), at("n1&#45;&gt;n2"), at("picture:1")
		if !(under < bench && bench < camera && camera < wire && wire < over) {
			t.Errorf("%s: SVG does not paint the background, the parts, the connection, then the overlay in that order:\n%s", style, svg)
		}
	}
}

// A picture at a URL is not handed to Graphviz as a file: it is refused with
// the reason, and the DOT form draws no image node for it.
func TestPictureAtURLIsRefusedNotDrawn(t *testing.T) {
	rendering := render(t, "pictures.sysml", "Site::remoteView")
	if len(rendering.Pictures) != 0 {
		t.Fatalf("pictures = %+v, want none", rendering.Pictures)
	}
	notice := "Picture annotation of view Site::remoteView is not applied: location of Picture is a URL, not the path of a file the drawing tools can read"
	if len(rendering.Notices) != 1 || rendering.Notices[0] != notice {
		t.Fatalf("notices = %q, want [%q]", rendering.Notices, notice)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dot, "image=") || strings.Contains(dot, "example.org") {
		t.Errorf("DOT hands the URL to Graphviz:\n%s", dot)
	}
	if !strings.Contains(dot, "// not represented: "+notice) {
		t.Errorf("DOT drops the picture silently:\n%s", dot)
	}
}

func TestPictureAtOtherSchemeIsRefusedByForms(t *testing.T) {
	rendering := render(t, "pictures.sysml", "Site::schemeView")
	if len(rendering.Pictures) != 1 || rendering.Pictures[0].Location != "javascript:alert(1)" {
		t.Fatalf("pictures = %+v, want the model's javascript: location", rendering.Pictures)
	}
	notice := "1 picture(s) not drawn: javascript:alert(1) at (0, 0) size 400×300; remote pictures are not drawn"
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if strings.Contains(dot, "image=") || !strings.Contains(dot, "// not represented: "+notice) {
		t.Errorf("DOT did not refuse and notice the scheme picture:\n%s", dot)
	}
	mermaid := rendering.Mermaid()
	if strings.Contains(mermaid, "img: \"javascript:alert(1)") || !strings.Contains(mermaid, "%% not represented: "+notice) {
		t.Errorf("Mermaid did not refuse and notice the scheme picture:\n%s", mermaid)
	}
}

// A picture's path is its location resolved against the document's directory,
// with an absolute path left as it is.
func TestPicturePath(t *testing.T) {
	abs := filepath.Join(string(filepath.Separator), "srv", "model", "images", "a.png")
	cases := []struct {
		picture Picture
		want    string
	}{
		{Picture{Location: "images/a.png", Dir: filepath.Join("srv", "model")}, filepath.Join("srv", "model", "images", "a.png")},
		{Picture{Location: "images/a.png"}, "images/a.png"},
		{Picture{Location: abs, Dir: "elsewhere"}, abs},
	}
	for _, c := range cases {
		if got := c.picture.Path(); got != c.want {
			t.Errorf("%+v.Path() = %q, want %q", c.picture, got, c.want)
		}
	}
}
