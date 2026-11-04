package view

import (
	"strings"
	"testing"
)

// A node whose Layout hides compartment rows ends with the notation's
// ellipsis in every form that lists compartments, and not when the rendering
// shows no compartments at all.
func TestElidedCompartmentsCarryTheEllipsis(t *testing.T) {
	for _, form := range []Form{FormDot, FormMermaid, FormPlantUML, FormD2, FormDrawio, FormText} {
		selective, err := render(t, "suppressed-compartments.sysml", "CameraViews::selective").Write(form)
		if err != nil {
			t.Fatalf("write selective %s: %v", form, err)
		}
		if !strings.Contains(string(selective), ellipsisMark) {
			t.Errorf("%s: a node with hidden compartments lacks the %s mark", form, ellipsisMark)
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
