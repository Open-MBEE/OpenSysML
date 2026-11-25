package view

import (
	"path/filepath"
	"strings"
	"testing"
)

// A Layout's show… switches hide compartments of one symbol: hidden ones are
// neither listed nor counted as clipped, the rest keep the fit and clipping
// rules, and a rendering that shows no compartments still shows none.
func TestGoldenSuppressedCompartments(t *testing.T) {
	for _, view := range []string{"selective", "collapsed", "cramped", "roomy", "bare"} {
		rendering := render(t, "suppressed-compartments.sysml", "CameraViews::"+view)
		for _, form := range []Form{FormDot, FormMermaid, FormPlantUML, FormD2, FormText} {
			out, err := rendering.Write(form)
			if err != nil {
				t.Fatalf("write %s %s: %v", view, form, err)
			}
			checkGolden(t, filepath.Join("testdata", "suppressed-compartments-"+view+"."+string(form)+".golden"), out)
		}
	}
}

func TestSuppressedCompartmentsAreNotListedNorClipped(t *testing.T) {
	dot := func(view string) string {
		out, err := render(t, "suppressed-compartments.sysml", "CameraViews::"+view).Write(FormDot)
		if err != nil {
			t.Fatalf("write %s: %v", view, err)
		}
		return string(out)
	}
	selective := withoutViewFrame(dot("selective"))
	for _, row := range []string{"resolution", "exposure", "mount", "data", "focus"} {
		if !strings.Contains(selective, row) {
			t.Errorf("selective lacks the row %s of a compartment left on", row)
		}
	}
	for _, row := range []string{"lens", "sensor", "capture", "frame", ">items<", ">actions<"} {
		if strings.Contains(selective, row) {
			t.Errorf("selective lists %s, which its Layout hides", row)
		}
	}
	if strings.Contains(selective, "compartment row(s) with no room") {
		t.Errorf("selective reports clipped rows:\n%s", selective)
	}
	collapsed := withoutViewFrame(dot("collapsed"))
	for _, text := range []string{"resolution", "mount", "data", "focus", "capture", "frame", "compartment row(s) with no room"} {
		if strings.Contains(collapsed, text) {
			t.Errorf("collapsed writes %q, though every compartment is hidden", text)
		}
	}
	cramped := dot("cramped")
	if !strings.Contains(cramped, "7 compartment row(s) with no room in the stated size of their node's box, left unwritten") {
		t.Errorf("cramped does not report the 7 rows of the compartments left on as clipped:\n%s", cramped)
	}
	if strings.Contains(cramped, "lens") || strings.Contains(cramped, "sensor") {
		t.Errorf("cramped lists a hidden part:\n%s", cramped)
	}
	roomy := dot("roomy")
	for _, row := range []string{"resolution", "exposure", "mount", "data", "focus", "capture", "frame"} {
		if !strings.Contains(roomy, row) {
			t.Errorf("roomy lacks the row %s of a compartment left on", row)
		}
	}
	if strings.Contains(roomy, "lens") || strings.Contains(roomy, "sensor") || strings.Contains(roomy, "compartment row(s) with no room") {
		t.Errorf("roomy lists a hidden part or clips:\n%s", roomy)
	}
	bare := dot("bare")
	for _, text := range []string{"resolution", "mount", "data", "lens", "compartment row(s)"} {
		if strings.Contains(bare, text) {
			t.Errorf("bare writes %q under a rendering without compartments", text)
		}
	}
}

// Hiding a compartment hides the rows of its keyword wherever a modifier
// lists them: `showAttributes = false` hides an `end attribute` under `ends`.
func TestHiddenAttributesHideTheEndAttributes(t *testing.T) {
	rendering := renderSource(t, "M::V::links", `package M {
	private import ScalarValues::Real;
	part def Sensor;
	connection def Link {
		end attribute gain : Real;
		end part sensor : Sensor;
		attribute weight : Real;
	}
	package V {
		private import Views::*;
		private import DiagramLayout::*;
		view links {
			expose M::Link;
			metadata Layout about M::Link { x = 10; y = 10; width = 240; height = 120; showAttributes = false; }
			render asTreeDiagram;
		}
	}
}`)
	out, err := rendering.Write(FormText)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{"attribute gain", "attribute weight"} {
		if strings.Contains(out, hidden) {
			t.Errorf("a hidden attribute is listed, %q:\n%s", hidden, out)
		}
	}
	if !strings.Contains(out, "end part sensor") || !strings.Contains(out, ellipsisMark) {
		t.Errorf("the end part or the ellipsis is missing:\n%s", out)
	}
}
