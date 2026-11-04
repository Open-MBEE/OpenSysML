package view

import (
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
