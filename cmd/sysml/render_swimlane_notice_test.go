package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Every form, and an import, says on stderr which swimlanes are stated too near for the frame
// between their flows, and what it covers; lanes far enough apart, or in one flow, say nothing.
func TestRenderSaysWhichStackedSwimlanesAFlowFrameCovers(t *testing.T) {
	binary := buildCLI(t)
	model, err := filepath.Abs("../../internal/ir/view/testdata/action-swimlane-layout-stacked.sysml")
	if err != nil {
		t.Fatal(err)
	}
	const notice = "swimlane line.packer is placed 48 px below swimlane line.robot, in another flow: " +
		"the frame of LaneStacked::runB needs 92 px there and is drawn over swimlane line.robot (142×44 px), " +
		"its step load (64×28 px), the frame of LaneStacked::runA (174×60 px)"
	for _, form := range []string{"text", "mermaid", "dot", "plantuml", "d2", "drawio"} {
		for _, c := range []struct {
			view    string
			covered bool
		}{{"closeView", true}, {"stackedView", false}, {"oneFlowView", false}} {
			t.Run(form+"/"+c.view, func(t *testing.T) {
				got := runFiles(t, binary, []string{model}, "-render", "LaneStacked::"+c.view, "-render-form", form)
				if got.status != exitHolds {
					t.Fatalf("exit status = %d\n%s", got.status, got.output())
				}
				if said := strings.Contains(got.stderr, "note: "+notice); said != c.covered {
					t.Errorf("stderr names the covered lane, step and frame: %v, want %v:\n%s", said, c.covered, got.stderr)
				}
				if !c.covered && strings.Contains(got.stderr, "is drawn over") {
					t.Errorf("lanes drawn clear are reported as covered:\n%s", got.stderr)
				}
			})
		}
	}
	for _, c := range []struct {
		view    string
		covered bool
	}{{"closeView", true}, {"stackedView", false}, {"oneFlowView", false}} {
		t.Run("import/"+c.view, func(t *testing.T) {
			dir := t.TempDir()
			source, err := os.ReadFile(model)
			if err != nil {
				t.Fatal(err)
			}
			copied := filepath.Join(dir, "stacked.sysml")
			if err := os.WriteFile(copied, source, 0o600); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(dir, "view.drawio")
			if got := runFiles(t, binary, []string{copied}, "-render", "LaneStacked::"+c.view, "-render-form", "drawio", "-o", file); got.status != exitHolds {
				t.Fatalf("export: exit status = %d\n%s", got.status, got.output())
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			// Drag the last step of the view's last flow, which is in no lane.
			step := map[string]string{"closeView": "LaneStacked::runB::post", "stackedView": "LaneStacked::runB::post", "oneFlowView": "LaneStacked::runAll::pack"}[c.view]
			cell := regexp.MustCompile(`(?s)(<object id="[^"]*" [^>]*fqn="` + step + `"[^>]*>.*?<mxGeometry x=")([^"]*)(")`)
			m := cell.FindStringSubmatch(string(data))
			if m == nil {
				t.Fatalf("no cell of %s in the export:\n%s", step, data)
			}
			x, _ := strconv.ParseFloat(m[2], 64)
			moved := cell.ReplaceAllString(string(data), "${1}"+strconv.FormatFloat(x+8, 'f', -1, 64)+"${3}")
			if err := os.WriteFile(file, []byte(moved), 0o600); err != nil {
				t.Fatal(err)
			}
			got := runFiles(t, binary, []string{copied}, "-import-diagram", file, "-dry-run")
			if got.status != exitHolds {
				t.Fatalf("import: exit status = %d\n%s", got.status, got.output())
			}
			if said := strings.Contains(got.stderr, file+": "+notice); said != c.covered {
				t.Errorf("the import names the covered lane, step and frame: %v, want %v:\n%s", said, c.covered, got.output())
			}
		})
	}
}
