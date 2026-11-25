package docrender

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

func TestHTMLRepeatedExpansionFoldNotice(t *testing.T) {
	var src strings.Builder
	src.WriteString("package P { private import Views::*; private import DiagramLayout::*; part def Leaf { part item; }")
	previous := "Leaf"
	for level := range 7 {
		name := fmt.Sprintf("D%d", level)
		fmt.Fprintf(&src, "part def %s { part a : %s; part b : %s; part c : %s; part d : %s; }", name, previous, previous, previous, previous)
		previous = name
	}
	fmt.Fprintf(&src, "part root : %s; view v { expose root; metadata Layout about root { x = 0; y = 0; } render asTreeDiagram; } }", previous)
	path := filepath.Join(t.TempDir(), "fold.sysml")
	if err := os.WriteFile(path, []byte(src.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture := loadRenderFixture(t, path)
	rendering, err := view.NewRenderer(fixture.model, fixture.resolver, nil).Render(fixture.symbol(t, "P::v"))
	if err != nil {
		t.Fatal(err)
	}
	if rendering.ExpansionFoldNotice == "" {
		t.Fatal("view must activate the default fold threshold")
	}
	want := "<p class=\"sysml-diagram-notice\"><em>" + html.EscapeString(rendering.ExpansionFoldNotice) + "</em></p>"
	const ordinary = "ordinary notice stays in source comments"
	rendering.Notices = append(rendering.Notices, ordinary)
	for _, form := range []view.Form{view.FormMermaid, view.FormDot, view.FormPlantUML, view.FormD2} {
		for _, svg := range []string{"", `<svg xmlns="http://www.w3.org/2000/svg"><text>drawing</text></svg>`} {
			w := &htmlWriter{forms: DiagramOptions{Form: form}, opts: HTMLOptions{DiagramSVG: []string{svg}}}
			if err := w.writeFigure("", "fold", captionOf(""), rendering, view.Options{}); err != nil {
				t.Fatal(err)
			}
			got := w.b.String()
			if !strings.Contains(got, want) {
				t.Fatalf("%s SVG=%v lost the visible notice", form, svg != "")
			}
			if strings.Contains(got, "<em>"+ordinary+"</em>") {
				t.Fatal("ordinary notice was promoted into a paragraph")
			}
		}
	}
}
