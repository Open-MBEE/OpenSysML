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
	// A flow frame still over a swimlane after an import placed the flows is reported by every render.
	for _, c := range []struct {
		name, cell, axis string
		by               float64
		notice           string
	}{
		{"post", "LaneStacked::runB::post", "x", 8, "swimlane line.packer is placed 48 px below swimlane line.robot, in another flow: " +
			"the frame of LaneStacked::runB, which its Layout places, is drawn over swimlane line.robot (142×44 px), " +
			"its step load (64×28 px), the frame of LaneStacked::runA (174×60 px); " +
			"Layouts placing that frame and what it holds 60 px lower keep them clear"},
		{"runB", "LaneStacked::runB", "y", 10, "swimlane line.packer is placed 58 px below swimlane line.robot, in another flow: " +
			"the frame of LaneStacked::runB, which its Layout places, is drawn over swimlane line.robot (142×34 px), " +
			"its step load (64×18 px), the frame of LaneStacked::runA (174×50 px); " +
			"Layouts placing that frame and what it holds 50 px lower keep them clear"},
	} {
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
		if got := runFiles(t, binary, []string{copied}, "-render", "LaneStacked::closeView", "-render-form", "drawio", "-o", file); got.status != exitHolds {
			t.Fatalf("export: exit status = %d\n%s", got.status, got.output())
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		cell := regexp.MustCompile(`(?s)(<object id="[^"]*" [^>]*fqn="` + c.cell + `"[^>]*>.*?<mxGeometry [^>]*\b` + c.axis + `=")([^"]*)(")`)
		m := cell.FindStringSubmatch(string(data))
		if m == nil {
			t.Fatalf("no cell of %s in the export:\n%s", c.cell, data)
		}
		at, _ := strconv.ParseFloat(m[2], 64)
		moved := cell.ReplaceAllString(string(data), "${1}"+strconv.FormatFloat(at+c.by, 'f', -1, 64)+"${3}")
		if err := os.WriteFile(file, []byte(moved), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := runFiles(t, binary, []string{copied}, "-import-diagram", file); got.status != exitHolds {
			t.Fatalf("import: exit status = %d\n%s", got.status, got.output())
		}
		forms := []string{"text", "mermaid", "dot", "plantuml", "d2", "drawio"}
		for _, form := range forms {
			t.Run("imported/"+c.name+"/"+form, func(t *testing.T) {
				got := runFiles(t, binary, []string{copied}, "-render", "LaneStacked::closeView", "-render-form", form)
				if got.status != exitHolds {
					t.Fatalf("exit status = %d\n%s", got.status, got.output())
				}
				if !strings.Contains(got.stderr, "note: "+c.notice) {
					t.Errorf("stderr does not name the lane, step and frame still covered:\n%s", got.stderr)
				}
			})
		}
		// Following the notice, the frame and what it holds placed that much lower, clears it.
		lower := regexp.MustCompile(`px\), the frame of LaneStacked::runA \(174×(\d+) px\); Layouts placing that frame and what it holds (\d+) px lower`).FindStringSubmatch(c.notice)
		by, _ := strconv.ParseFloat(lower[2], 64)
		placed, err := os.ReadFile(copied)
		if err != nil {
			t.Fatal(err)
		}
		inCloseView := regexp.MustCompile(`(Layout about (?:line::packer|LaneStacked::runB(?:::\w+)?) \{ x = [\d.]+; y = )([\d.]+)(;[^\n]*\n)`)
		from := strings.Index(string(placed), "view closeView")
		shifted := string(placed[:from]) + inCloseView.ReplaceAllStringFunc(string(placed[from:]), func(line string) string {
			m := inCloseView.FindStringSubmatch(line)
			y, _ := strconv.ParseFloat(m[2], 64)
			return m[1] + strconv.FormatFloat(y+by, 'f', -1, 64) + m[3]
		})
		if err := os.WriteFile(copied, []byte(shifted), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Run("cleared/"+c.name, func(t *testing.T) {
			got := runFiles(t, binary, []string{copied}, "-render", "LaneStacked::closeView", "-render-form", "drawio")
			if got.status != exitHolds || strings.Contains(got.stderr, "is drawn over") {
				t.Errorf("the Layouts the notice asks for still draw the frame over the lane (status %d):\n%s", got.status, got.stderr)
			}
		})
	}
}

// A Layout may place a swimlane past the frame of its own flow: the notice names the frame only
// when it is covered, and what it asks for clears the lane as well.
func TestRenderClearsASwimlanePlacedPastItsFlowFrame(t *testing.T) {
	binary := buildCLI(t)
	source, err := os.ReadFile("../../internal/ir/view/testdata/action-swimlane-layout-stacked.sysml")
	if err != nil {
		t.Fatal(err)
	}
	const robot = "is drawn over swimlane line.robot (142×"
	for _, c := range []struct {
		name   string
		y      float64
		notice string
	}{
		{"frame and lane", 130, robot + "44 px), the frame of LaneStacked::runA (174×10 px); Layouts placing that frame and what it holds 44 px lower keep them clear"},
		{"lane alone", 150, robot + "24 px); Layouts placing that frame and what it holds 24 px lower keep them clear"},
	} {
		view := func(y float64) string {
			at := strconv.FormatFloat(y, 'f', -1, 64)
			lane := strconv.FormatFloat(y+100, 'f', -1, 64)
			return "\tview pastView : ActionFlowView {\n\t\texpose runA;\n\t\texpose runB;\n" +
				"\t\tmetadata Layout about runA { x = 300; y = 60; width = 200; height = 80; }\n" +
				"\t\tmetadata Layout about line::robot { x = 320; y = 120; }\n" +
				"\t\tmetadata Layout about runB { x = 304; y = " + at + "; }\n" +
				"\t\tmetadata Layout about line::packer { x = 320; y = " + lane + "; }\n\t}\n}\n"
		}
		render := func(t *testing.T, y float64) string {
			body := strings.TrimRight(string(source), "\n")
			model := filepath.Join(t.TempDir(), "past.sysml")
			if err := os.WriteFile(model, []byte(strings.TrimSuffix(body, "}")+view(y)), 0o600); err != nil {
				t.Fatal(err)
			}
			got := runFiles(t, binary, []string{model}, "-render", "LaneStacked::pastView", "-render-form", "text")
			if got.status != exitHolds {
				t.Fatalf("exit status = %d\n%s", got.status, got.output())
			}
			return got.stderr
		}
		t.Run(c.name, func(t *testing.T) {
			if stderr := render(t, c.y); !strings.Contains(stderr, c.notice) {
				t.Errorf("stderr does not say what clears the lane, want %q:\n%s", c.notice, stderr)
			}
			by, _ := strconv.ParseFloat(regexp.MustCompile(`holds (\d+) px lower`).FindStringSubmatch(c.notice)[1], 64)
			if stderr := render(t, c.y+by); strings.Contains(stderr, "is drawn over") {
				t.Errorf("the Layouts the notice asks for still draw the frame over the lane:\n%s", stderr)
			}
		})
	}
}
