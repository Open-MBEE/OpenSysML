package view

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A node whose Layout hides compartment rows ends with the notation's
// ellipsis in every form that lists compartments, and not when the rendering
// shows no compartments at all.
func TestElidedCompartmentsCarryTheEllipsis(t *testing.T) {
	for _, form := range []Form{FormDot, FormMermaid, FormPlantUML, FormD2, FormDrawio, FormText} {
		// A 33 pt stated box holds its head alone: DOT, which fits the label to the
		// stated size, has no room for the mark there and drops it as it drops rows.
		views := []string{"selective", "roomy", "collapsed", "cramped"}
		if form == FormDot {
			views = views[:2]
		}
		for _, view := range views {
			out, err := render(t, "suppressed-compartments.sysml", "CameraViews::"+view).Write(form)
			if err != nil {
				t.Fatalf("write %s %s: %v", view, form, err)
			}
			if !strings.Contains(string(out), ellipsisMark) {
				t.Errorf("%s %s: a node with hidden compartments lacks the %s mark", view, form, ellipsisMark)
			}
		}
		bare, err := render(t, "suppressed-compartments.sysml", "CameraViews::bare").Write(form)
		if err != nil {
			t.Fatalf("write bare %s: %v", form, err)
		}
		if strings.Contains(string(bare), ellipsisMark) {
			t.Errorf("%s: a rendering without compartments carries the %s mark", form, ellipsisMark)
		}
	}
	plain, err := render(t, "tree-compartments.sysml", "AmplifierViews::positioned").Write(FormDot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), ellipsisMark) {
		t.Errorf("a node with every row listed carries the %s mark", ellipsisMark)
	}
}

// A draw.io box whose stated height leaves no room under its header still
// keeps the ellipsis cell inside it, at its lower left.
func TestDrawioEllipsisStaysInsideAStatedBox(t *testing.T) {
	out, err := render(t, "suppressed-compartments.sysml", "CameraViews::collapsed").Write(FormDrawio)
	if err != nil {
		t.Fatal(err)
	}
	cell := regexp.MustCompile(`<mxCell id="([^"]+):ellipsis"[^>]*parent="([^"]+)"[^>]*>\s*<mxGeometry([^>]*)/>`).FindStringSubmatch(string(out))
	if cell == nil {
		t.Fatalf("no ellipsis cell:\n%s", out)
	}
	if cell[1] != cell[2] {
		t.Errorf("the ellipsis cell of %s is parented to %s", cell[1], cell[2])
	}
	y, height := drawioGeometryAttr(t, cell[3], "y"), drawioGeometryAttr(t, cell[3], "height")
	box := regexp.MustCompile(`<object id="` + regexp.QuoteMeta(cell[1]) + `"[^>]*>\s*<mxCell[^>]*>\s*<mxGeometry([^>]*)/>`).FindStringSubmatch(string(out))
	if box == nil {
		t.Fatalf("no box %s:\n%s", cell[1], out)
	}
	if stated := drawioGeometryAttr(t, box[1], "height"); y+height > stated {
		t.Errorf("the ellipsis cell at y=%v, height %v leaves the box's stated height %v", y, height, stated)
	}
}

// drawioGeometryAttr reads one numeric attribute of an mxGeometry's attribute text, 0 when absent.
func drawioGeometryAttr(t *testing.T, attrs, name string) float64 {
	t.Helper()
	m := regexp.MustCompile(` ` + name + `="([^"]*)"`).FindStringSubmatch(attrs)
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("%s=%q: %v", name, m[1], err)
	}
	return v
}

// A compartment row an edge ends at — the membership line folded to its
// owner's box — is greyed as the element inside a textual compartment: a
// gray font in DOT, a gray colour in PlantUML, italics in Mermaid; a row no
// edge ends at is written plain.
func TestGreyedRowsMarkFoldedEdgeEnds(t *testing.T) {
	rendering := render(t, "tree-compartments-most.sysml", "CompartmentDemoViews::controllers")
	options := Options{Compartments: CompartmentsMost}
	for _, tc := range []struct {
		form          Form
		greyed, plain string
	}{
		{FormDot, `<font color="gray">sensor : Sensor</font>`, `<td align="left">enabled : Boolean</td>`},
		{FormPlantUML, `<color:gray>sensor : Sensor</color>`, `\nenabled : Boolean\n`},
		{FormMermaid, "*sensor : Sensor*", "\nenabled : Boolean\n"},
	} {
		out, err := rendering.WriteWith(tc.form, options)
		if err != nil {
			t.Fatalf("write %s: %v", tc.form, err)
		}
		if !strings.Contains(string(out), tc.greyed) {
			t.Errorf("%s lacks the greyed row %q:\n%s", tc.form, tc.greyed, out)
		}
		if !strings.Contains(string(out), tc.plain) {
			t.Errorf("%s lacks the plain row %q:\n%s", tc.form, tc.plain, out)
		}
	}
	unfolded, err := rendering.Write(FormDot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(unfolded), `color="gray"`) {
		t.Errorf("an unfolded tree greys a node:\n%s", unfolded)
	}
}

// A stated DOT box takes the ellipsis only when the table it sets, padding
// included, still fits the box; the mark appears once the box has the room and
// stays as the box grows.
func TestEllipsisFitsTheStatedDOTBox(t *testing.T) {
	model, err := os.ReadFile("testdata/suppressed-compartments.sysml")
	if err != nil {
		t.Fatal(err)
	}
	var marked, unmarked int
	for height := 33; height <= 90; height++ {
		text := string(model) + fmt.Sprintf(`package TightViews {
	private import Views::*;
	private import DiagramLayout::*;
	view tight {
		expose Camera::Imaging;
		metadata Layout about Camera::Imaging {
			x = 10; y = 10; width = 200; height = %d;
			showAttributes = false; showParts = false; showReferences = false; showPorts = false;
			showConstraints = false; showItems = false; showActions = false;
		}
		render asTreeDiagram;
	}
}
`, height)
		out, err := renderSource(t, "TightViews::tight", text).Write(FormDot)
		if err != nil {
			t.Fatalf("height %d: %v", height, err)
		}
		label := dotNodeLabel(t, string(out), "Imaging")
		if got := dotHTMLLabelHeight(label, dotBaseFontSize(string(out))); got > float64(height) {
			t.Errorf("height %d: the label %s needs %.1f points", height, label, got)
		}
		if strings.Contains(label, ellipsisMark) {
			marked++
		} else if marked > 0 {
			t.Errorf("height %d: the mark fitted a shorter box but not this one", height)
		} else {
			unmarked++
		}
	}
	if marked == 0 || unmarked == 0 {
		t.Errorf("the sweep set %d labels with the mark and %d without; want both", marked, unmarked)
	}
}

// dotNodeLabel is the HTML-like label of the DOT node naming name.
func dotNodeLabel(t *testing.T, dot, name string) string {
	t.Helper()
	for _, line := range strings.Split(dot, "\n") {
		if start := strings.Index(line, "[label=<"); start >= 0 && strings.Contains(line, name) {
			if end := strings.LastIndex(line, ">,"); end > start {
				return line[start+len("[label=") : end+1]
			}
		}
	}
	t.Fatalf("no labelled node %s in\n%s", name, dot)
	return ""
}

// dotBaseFontSize is the font size the DOT output's nodes default to.
func dotBaseFontSize(dot string) float64 {
	if m := regexp.MustCompile(`node \[[^\]]*fontsize="?([0-9.]+)`).FindStringSubmatch(dot); m != nil {
		size, _ := strconv.ParseFloat(m[1], 64)
		return size
	}
	return 14
}

// dotHTMLLabelHeight measures a DOT HTML-like label as Graphviz lays it out: each
// line at its point size, each table row padded above and below.
func dotHTMLLabelHeight(label string, base float64) float64 {
	inner := strings.TrimSuffix(strings.TrimPrefix(label, "<"), ">")
	rows, padding := []string{inner}, 0.0
	if strings.HasPrefix(inner, "<table") {
		rows, padding = strings.Split(inner, "<tr>")[1:], 2*float64(dotCellPadding)
	}
	pointSize := regexp.MustCompile(`point-size="([0-9.]+)"`)
	var height float64
	for _, row := range rows {
		height += padding
		for _, line := range strings.Split(row, "<br/>") {
			size := base
			if m := pointSize.FindStringSubmatch(line); m != nil {
				size, _ = strconv.ParseFloat(m[1], 64)
			}
			height += dotLineHeight(size)
		}
	}
	return height
}
