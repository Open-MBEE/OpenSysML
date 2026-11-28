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
		"the frame of LaneStacked::runB needs 108 px there and is drawn over swimlane line.robot (142×44 px), " +
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

// Swimlanes placed past their flow's frame, against each other or over each other: the notice
// names what is covered, states no size of zero or less, and what it asks for clears the lane.
func TestRenderClearsSwimlanesPlacedPastAFrameOrOverEachOther(t *testing.T) {
	binary := buildCLI(t)
	source, err := os.ReadFile("../../internal/ir/view/testdata/action-swimlane-layout-stacked.sysml")
	if err != nil {
		t.Fatal(err)
	}
	const pinnedA = "\t\tmetadata Layout about runA { x = 300; y = 60; width = 200; height = 80; }\n"
	for _, c := range []struct {
		name   string
		robot  string  // the Layout of the lane covered
		frameA string  // the Layout of its flow, if any
		frameB float64 // the y of the covering flow's Layout, if any
		packer float64 // the y of the covering flow's lane
		notice string
	}{
		{"placed frame over the lane and its frame", "x = 320; y = 120;", pinnedA, 130, 230,
			"swimlane line.packer is placed 56 px below swimlane line.robot, in another flow: the frame of LaneStacked::runB, which its Layout places, " +
				"is drawn over swimlane line.robot (142×44 px), the frame of LaneStacked::runA (174×10 px); Layouts placing that frame and what it holds 44 px lower keep them clear"},
		{"placed frame over the lane alone", "x = 320; y = 120;", pinnedA, 150, 250,
			"is drawn over swimlane line.robot (142×24 px); Layouts placing that frame and what it holds 24 px lower keep them clear"},
		{"lanes over each other", "x = 320; y = 120; width = 142; height = 212;", "", 0, 330,
			"swimlane line.packer is placed over swimlane line.robot (142×2 px), in another flow: the frame of LaneStacked::runB " +
				"is drawn over swimlane line.robot (142×94 px), the frame of LaneStacked::runA (174×110 px); Layouts placing the swimlanes of LaneStacked::runB 110 px lower keep them clear"},
		{"lanes against each other", "x = 320; y = 120; width = 142; height = 210;", "", 0, 330,
			"swimlane line.packer is placed directly below swimlane line.robot, in another flow: the frame of LaneStacked::runB needs 108 px there " +
				"and is drawn over swimlane line.robot (142×92 px), the frame of LaneStacked::runA (174×108 px); a Layout that far apart keeps them clear"},
	} {
		render := func(t *testing.T, lower float64) string {
			y := func(at float64) string { return strconv.FormatFloat(at+lower, 'f', -1, 64) }
			view := "\tview pastView : ActionFlowView {\n\t\texpose runA;\n\t\texpose runB;\n" + c.frameA +
				"\t\tmetadata Layout about line::robot { " + c.robot + " }\n"
			if c.frameB != 0 {
				view += "\t\tmetadata Layout about runB { x = 304; y = " + y(c.frameB) + "; }\n"
			}
			view += "\t\tmetadata Layout about line::packer { x = 320; y = " + y(c.packer) + "; }\n\t}\n}\n"
			model := filepath.Join(t.TempDir(), "past.sysml")
			body := strings.TrimSuffix(strings.TrimRight(string(source), "\n"), "}")
			if err := os.WriteFile(model, []byte(body+view), 0o600); err != nil {
				t.Fatal(err)
			}
			got := runFiles(t, binary, []string{model}, "-render", "LaneStacked::pastView", "-render-form", "text")
			if got.status != exitHolds {
				t.Fatalf("exit status = %d\n%s", got.status, got.output())
			}
			return got.stderr
		}
		t.Run(c.name, func(t *testing.T) {
			stderr := render(t, 0)
			if !strings.Contains(stderr, c.notice) {
				t.Errorf("stderr does not say what clears the lane, want %q:\n%s", c.notice, stderr)
			}
			if none := regexp.MustCompile(`(^|[^\d.])(-[\d.]+|0) px`).FindString(stderr); none != "" {
				t.Errorf("a notice states a size of zero or less (%q):\n%s", none, stderr)
			}
			by, _ := strconv.ParseFloat(regexp.MustCompile(`(\d+) px (?:lower|there)`).FindStringSubmatch(c.notice)[1], 64)
			if stderr := render(t, by); strings.Contains(stderr, "is drawn over") {
				t.Errorf("the Layouts the notice asks for still draw the frame over the lane:\n%s", stderr)
			}
		})
	}
}

// A flow no Layout places goes on the grid, which no Layout moves: it starts under all a
// placed flow draws, so its frame is never over a lane placed past that flow's frame.
func TestRenderGridsAFlowUnderASwimlanePlacedPastItsFrame(t *testing.T) {
	binary := buildCLI(t)
	source, err := os.ReadFile("../../internal/ir/view/testdata/action-swimlane-layout-stacked.sysml")
	if err != nil {
		t.Fatal(err)
	}
	view := "\tview gridView : ActionFlowView {\n\t\texpose runA;\n\t\texpose runB;\n" +
		"\t\tmetadata Layout about runA { x = 20; y = 60; width = 200; height = 80; }\n" +
		"\t\tmetadata Layout about line::robot { x = 40; y = 120; width = 142; height = 120; }\n" +
		"\t\tmetadata Layout about line::packer { x = 40; y = 330; }\n" +
		"\t\tmetadata Layout about runB::post { x = 60; y = 480; }\n\t}\n}\n"
	model := filepath.Join(t.TempDir(), "grid.sysml")
	body := strings.TrimSuffix(strings.TrimRight(string(source), "\n"), "}")
	if err := os.WriteFile(model, []byte(body+view), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, form := range []string{"text", "drawio"} {
		got := runFiles(t, binary, []string{model}, "-render", "LaneStacked::gridView", "-render-form", form)
		if got.status != exitHolds {
			t.Fatalf("%s: exit status = %d\n%s", form, got.status, got.output())
		}
		if strings.Contains(got.stderr, "is drawn over") {
			t.Errorf("%s: the flow on the grid is drawn over a placed swimlane:\n%s", form, got.stderr)
		}
	}
}

// The gap a notice asks for between two swimlanes no flow frame is placed around clears all
// it names, the other flow's frame too, whichever lane's notice is followed.
func TestRenderClearsFlowFramesAtTheSwimlaneGapANoticeAsksFor(t *testing.T) {
	binary := buildCLI(t)
	source, err := os.ReadFile("../../internal/ir/view/testdata/action-swimlane-layout-stacked.sysml")
	if err != nil {
		t.Fatal(err)
	}
	body := strings.TrimSuffix(strings.TrimRight(string(source), "\n"), "}")
	frame := regexp.MustCompile(`label="«action»&#xA;run[AB]"[^>]*>\s*<mxCell[^>]*>\s*<mxGeometry x="([-\d.]+)" y="([-\d.]+)" width="([\d.]+)" height="([\d.]+)"`)
	asked := regexp.MustCompile(`swimlane line\.(\w+) is placed (?:([\d.]+) px|directly) (below|above|right of|left of) [^\n]* needs ([\d.]+) px there`)
	// render draws the two flows with the packer lane at x, y and returns the notices and how far the flow frames cross.
	render := func(t *testing.T, robot string, x, y float64) (string, float64, float64) {
		at := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
		view := "\tview gapView : ActionFlowView {\n\t\texpose runA;\n\t\texpose runB;\n" +
			"\t\tmetadata Layout about line::robot { " + robot + " }\n" +
			"\t\tmetadata Layout about line::packer { x = " + at(x) + "; y = " + at(y) + "; }\n\t}\n}\n"
		model := filepath.Join(t.TempDir(), "gap.sysml")
		if err := os.WriteFile(model, []byte(body+view), 0o600); err != nil {
			t.Fatal(err)
		}
		got := runFiles(t, binary, []string{model}, "-render", "LaneStacked::gapView", "-render-form", "drawio")
		if got.status != exitHolds {
			t.Fatalf("exit status = %d\n%s", got.status, got.output())
		}
		frames := frame.FindAllStringSubmatch(got.stdout, -1)
		if len(frames) != 2 {
			t.Fatalf("drawn flow frames = %d, want 2:\n%s", len(frames), got.stdout)
		}
		var box [2][4]float64
		for i, f := range frames {
			for j := range box[i] {
				box[i][j], _ = strconv.ParseFloat(f[j+1], 64)
			}
		}
		a, b := box[0], box[1]
		wide := min(a[0]+a[2], b[0]+b[2]) - max(a[0], b[0])
		high := min(a[1]+a[3], b[1]+b[3]) - max(a[1], b[1])
		return got.stderr, wide, high
	}
	for _, c := range []struct {
		name    string
		robot   string
		x, y    float64
		notices int
	}{
		{"below", "x = 320; y = 120;", 320, 330, 1},
		{"directly below, a notice for each frame", "x = 320; y = 120; width = 142; height = 210;", 320, 330, 2},
		{"above", "x = 320; y = 120;", 320, -80, 1},
		{"right of", "x = 320; y = 120;", 480, 120, 1},
		{"right of, a notice for each frame", "x = 320; y = 120;", 470, 200, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			stderr, _, _ := render(t, c.robot, c.x, c.y)
			notices := asked.FindAllStringSubmatch(stderr, -1)
			if len(notices) != c.notices {
				t.Fatalf("notices asking for a gap = %d, want %d:\n%s", len(notices), c.notices, stderr)
			}
			for _, n := range notices {
				stated, _ := strconv.ParseFloat(n[2], 64)
				needed, _ := strconv.ParseFloat(n[4], 64)
				by := needed - stated
				// The packer lane moves away from the robot lane, whichever of the two the notice is about.
				way := map[string][2]float64{"below": {0, by}, "above": {0, -by}, "right of": {by, 0}, "left of": {-by, 0}}[n[3]]
				dx, dy := way[0], way[1]
				if n[1] == "robot" {
					dx, dy = -dx, -dy
				}
				stderr, wide, high := render(t, c.robot, c.x+dx, c.y+dy)
				if strings.Contains(stderr, "is drawn over") {
					t.Errorf("%g px apart, as the notice for %s asks, a frame is still drawn over a lane:\n%s", needed, n[1], stderr)
				}
				if wide > 0 && high > 0 {
					t.Errorf("%g px apart, as the notice for %s asks, the flow frames cross by %g×%g px", needed, n[1], wide, high)
				}
			}
		})
	}
}
