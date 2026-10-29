package view

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The compressed fixture is the layout golden as draw.io desktop 32.6.1 saved
// it after File › Properties › Compressed and a drag of the tank to the right.
func TestReadDrawioCompressedDesktopSave(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "layout-desktop.drawio"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `compressed="true"`) || strings.Contains(string(data), "<mxGraphModel") {
		t.Fatalf("the fixture is not a compressed save:\n%s", data)
	}
	file, err := ReadDrawio(data, "")
	if err != nil {
		t.Fatal(err)
	}
	if file.View != layoutCase.view || file.PageWidth != 1200 || file.PageHeight != 800 {
		t.Errorf("view %q page %vx%v, want %s 1200x800", file.View, file.PageWidth, file.PageHeight, layoutCase.view)
	}
	// Dragged past the loop's border, the tank left its container: draw.io
	// re-parented it to the page, and the import must not care.
	tank := file.Cell("n2")
	if tank == nil || tank.FQN != "Plant::Loop::tank" || tank.Parent != "1" {
		t.Fatalf("tank = %+v", tank)
	}
	rendering, options := renderWithSites(t, layoutCase.file, layoutCase.view)
	imp, err := rendering.ImportDrawio(file, options)
	if err != nil {
		t.Fatal(err)
	}
	edit := editOf(t, imp, DrawioEditLayout, "n2")
	if edit.FQN != "Plant::Loop::tank" || edit.Layout.X <= 256 || edit.Layout.Y != 218 {
		t.Errorf("layout = %s %+v, want the tank moved right from (256, 218)", edit.FQN, edit.Layout)
	}
	if len(imp.Unmatched)+len(imp.Unknown)+len(imp.Deleted)+len(imp.Notices) != 0 {
		t.Errorf("reported %v %d %v %v, want nothing", imp.Unmatched, len(imp.Unknown), imp.Deleted, imp.Notices)
	}
}

func TestReadDrawioRejectsMalformedFiles(t *testing.T) {
	cases := map[string]string{
		"not xml":         "hello",
		"truncated":       `<mxfile><diagram name="x"><mxGraphModel><root><mxCell id="0"/>`,
		"not an mxfile":   `<svg/>`,
		"no page":         `<mxfile></mxfile>`,
		"bad compressed":  `<mxfile><diagram name="x">not base64 at all!</diagram></mxfile>`,
		"bad geometry":    `<mxfile><diagram name="x"><mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/><mxCell id="a" vertex="1" parent="1"><mxGeometry x="wide" as="geometry"/></mxCell></root></mxGraphModel></diagram></mxfile>`,
		"duplicate id":    `<mxfile><diagram name="x"><mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/><mxCell id="a" vertex="1" parent="1"/><mxCell id="a" vertex="1" parent="1"/></root></mxGraphModel></diagram></mxfile>`,
		"parent cycle":    `<mxfile><diagram name="x"><mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/><mxCell id="a" vertex="1" parent="b"><mxGeometry x="1" as="geometry"/></mxCell><mxCell id="b" vertex="1" parent="a"><mxGeometry x="1" as="geometry"/></mxCell></root></mxGraphModel></diagram></mxfile>`,
		"parent not cell": `<mxfile><diagram name="x"><mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/><mxCell id="a" vertex="1" parent="zz"><mxGeometry x="1" as="geometry"/></mxCell></root></mxGraphModel></diagram></mxfile>`,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ReadDrawio([]byte(data), "")
			var fileErr *DrawioFileError
			if !errors.As(err, &fileErr) {
				t.Fatalf("err = %v, want a *DrawioFileError", err)
			}
		})
	}
}

// drawioTestPage is one <diagram> whose cells carry the view attribute given.
func drawioTestPage(name, view string) string {
	attr := ""
	if view != "" {
		attr = ` view="` + view + `"`
	}
	return `<diagram name="` + name + `"><mxGraphModel pageWidth="100" pageHeight="100"><root><mxCell id="0"/><mxCell id="1" parent="0"/>` +
		`<object id="n0" label="a" fqn="P::a" kind="part"` + attr + `><mxCell vertex="1" parent="1"><mxGeometry x="1" y="2" width="3" height="4" as="geometry"/></mxCell></object>` +
		`</root></mxGraphModel></diagram>`
}

func TestReadDrawioSelectsThePageOfTheView(t *testing.T) {
	two := `<mxfile>` + drawioTestPage("first", "V::one") + drawioTestPage("second", "V::two") + `</mxfile>`
	file, err := ReadDrawio([]byte(two), "V::two")
	if err != nil {
		t.Fatal(err)
	}
	if file.Page != "second" || file.View != "V::two" || len(file.Pages) != 2 {
		t.Errorf("page %q view %q pages %v", file.Page, file.View, file.Pages)
	}
	var viewErr *DrawioViewError
	if _, err := ReadDrawio([]byte(two), ""); !errors.As(err, &viewErr) || !strings.Contains(err.Error(), "name the view") {
		t.Errorf("two pages without a view: %v", err)
	}
	if _, err := ReadDrawio([]byte(two), "V::three"); !errors.As(err, &viewErr) || !strings.Contains(err.Error(), "V::one, V::two") {
		t.Errorf("a view no page was rendered from: %v", err)
	}
	twice := `<mxfile>` + drawioTestPage("first", "V::one") + drawioTestPage("again", "V::one") + `</mxfile>`
	if _, err := ReadDrawio([]byte(twice), "V::one"); !errors.As(err, &viewErr) || !strings.Contains(err.Error(), "2 pages") {
		t.Errorf("two pages of one view: %v", err)
	}
	// A page without semantic cells is of the view its name states.
	unviewed := `<mxfile>` + drawioTestPage("V::one", "") + `</mxfile>`
	if file, err := ReadDrawio([]byte(unviewed), "V::one"); err != nil || file.View != "V::one" {
		t.Errorf("cells without a view on a page named after it: %v %v", file, err)
	}
	if _, err := ReadDrawio([]byte(unviewed), "V::two"); !errors.As(err, &viewErr) || !strings.Contains(err.Error(), "V::one") {
		t.Errorf("cells without a view on a page named after another: %v", err)
	}
	unnamed := `<mxfile>` + strings.Replace(drawioTestPage("", ""), ` name=""`, "", 1) + `</mxfile>`
	if _, err := ReadDrawio([]byte(unnamed), "V::one"); !errors.As(err, &viewErr) || !strings.Contains(err.Error(), "no view attribute") {
		t.Errorf("cells without a view on an unnamed page: %v", err)
	}
}

func TestReadDrawioRefusesPagesInflatingPastTheLimit(t *testing.T) {
	var deflated bytes.Buffer
	w, err := flate.NewWriter(&deflated, flate.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(bytes.Repeat([]byte("<"), drawioInflateLimit+1)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	page := `<mxfile><diagram name="x">` + base64.StdEncoding.EncodeToString(deflated.Bytes()) + `</diagram></mxfile>`
	_, err = ReadDrawio([]byte(page), "")
	var fileErr *DrawioFileError
	if !errors.As(err, &fileErr) || !strings.Contains(err.Error(), "inflates past 64 MiB") {
		t.Fatalf("err = %v, want the inflate limit", err)
	}
}

func TestReadDrawioKeepsUnknownCellsApart(t *testing.T) {
	data := `<mxfile>` + strings.Replace(drawioTestPage("p", "V::one"), "</root>",
		`<mxCell id="hand" value="todo" style="rounded=1;fillColor=#FFF;" vertex="1" parent="n0"><mxGeometry x="10" y="10" width="8" height="8" as="geometry"/></mxCell></root>`, 1) + `</mxfile>`
	file, err := ReadDrawio([]byte(data), "V::one")
	if err != nil {
		t.Fatal(err)
	}
	hand := file.Cell("hand")
	if hand == nil || hand.FQN != "" || hand.Value != "todo" || hand.Style["fillColor"] != "#FFF" || hand.Style["rounded"] != "1" {
		t.Fatalf("hand = %+v", hand)
	}
	if hand.Geometry.X != 11 || hand.Geometry.Y != 12 {
		t.Errorf("hand at %+v, want its parent's (1, 2) added", hand.Geometry)
	}
	if file.Cell("0") != nil || file.Cell("1") != nil {
		t.Errorf("the root cells are listed")
	}
}
