package view

import (
	"context"
	"html"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRepeatedExpansionFoldSVGNotice(t *testing.T) {
	dot, err := exec.LookPath("dot")
	if err != nil {
		t.Skip("dot is not installed")
	}
	src := strings.Replace(sharedQueryModel(3, 2), "metadata Layout about root { x = 0; y = 0; }", "", 1)
	r, idx := loadSources(t, []string{"fold.svg.sysml"}, [][]byte{[]byte(src)})
	r.expansionFoldThreshold = 2
	got, err := r.Render(lookup(t, idx, "P::v"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ExpansionFoldNotice == "" {
		t.Fatal("view must fold")
	}
	const ordinary = "ordinary notice stays in source comments"
	got.Notices = append(got.Notices, ordinary)
	for _, style := range []DrawingStyle{StylePilot, StyleCameo} {
		source, err := got.DOTWith(Options{Style: style})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		cmd := exec.CommandContext(ctx, dot, "-Tsvg")
		cmd.Stdin = strings.NewReader(source)
		svg, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("dot: %v\n%s", err, svg)
		}
		if !strings.Contains(html.UnescapeString(string(svg)), got.ExpansionFoldNotice) {
			t.Fatalf("%s SVG lost the fold notice", style)
		}
		if strings.Contains(string(svg), ordinary) {
			t.Fatal("ordinary notice was promoted into the drawing")
		}
	}
}
