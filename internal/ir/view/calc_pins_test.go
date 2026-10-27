package view

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// renderCalc draws one calculation of calc-action-view.sysml as an action rendering.
func renderCalc(t *testing.T, fqn string) *Rendering {
	t.Helper()
	r, idx := loadFixture(t, "calc-action-view.sysml")
	rendering, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, fqn)}, KindAction, "#action")
	if err != nil {
		t.Fatalf("render %s: %v", fqn, err)
	}
	return rendering
}

// A calculation is drawn by the action rendering as the action it is: one node
// with its directed parameters as pins, the result parameter an out pin named
// result when the `return` declares no name.
func TestCalcRendersAsActionWithParameterPins(t *testing.T) {
	cases := []struct {
		fqn, kind string
		pins      []string
		steps     int
	}{
		{"Calcs::add", "calc", []string{"in x", "in y", "out result"}, 0},
		{"Calcs::Add", "calc def", []string{"in x", "in y", "out r"}, 0},
		{"Calcs::Totaliser::total", "calc", []string{"in x", "in y", "out r"}, 0},
		{"Calcs::Stepwise", "calc def", []string{"in load", "out result"}, 2},
	}
	for _, tc := range cases {
		rendering := renderCalc(t, tc.fqn)
		wantNoNotices(t, rendering)
		if len(rendering.Roots) != 1 {
			t.Fatalf("%s: %d roots, want the calculation alone", tc.fqn, len(rendering.Roots))
		}
		root := rendering.Roots[0]
		if root.Kind != tc.kind {
			t.Errorf("%s: kind %q, want %q", tc.fqn, root.Kind, tc.kind)
		}
		if got := pinTexts(root); !slices.Equal(got, tc.pins) {
			t.Errorf("%s: pins %q, want %q", tc.fqn, got, tc.pins)
		}
		if len(root.Children) != tc.steps {
			t.Errorf("%s: %d steps, want %d", tc.fqn, len(root.Children), tc.steps)
		}
	}
}

// The text form names the anonymous result pin result, as the pilot labels it,
// and the Mermaid and DOT forms draw the same pins.
func TestCalcResultPinIsNamedInEveryForm(t *testing.T) {
	rendering := renderCalc(t, "Calcs::add")
	if text := rendering.textWith(fullNotation); !strings.Contains(text, "calc Calcs::add (declares no nodes)\n  in x\n  in y\n  out result\n") {
		t.Errorf("text lacks the pins:\n%s", text)
	}
	dot, err := rendering.DOTWith(fullNotation)
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{">x</font>", ">y</font>", ">result</font>"} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %s:\n%s", want, dot)
		}
	}
	if mermaid := rendering.MermaidWith(fullNotation); !strings.Contains(mermaid, "**add**") {
		t.Errorf("Mermaid lacks the calc node:\n%s", mermaid)
	}
}
