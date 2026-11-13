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
	if text := rendering.textWith(fullNotation); !strings.Contains(text, "calc Calcs::add\n  result\n    x + y\n  in x\n  in y\n  out result\n") {
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

// A mixed rendering draws the calc usages a behavior's body holds beneath its
// action node, once each, and a directed calc usage as the parameter pin it is.
func TestMixedRenderingKeepsNestedCalcsUnderTheirOwner(t *testing.T) {
	const model = `package Nested {
  private import ScalarValues::*;
  part def Wheel;
  calc def Outer { calc inner { return r : Integer = 1; } }
  action def Act { calc scale { in x : Integer; return y : Integer = x; } action step; }
  action def Param { in calc f { in x : Integer; return : Integer; } }
  calc def Shaped { in calc g { return : Integer; } return : Integer; }
}
`
	r, idx := loadSources(t, []string{"nested-calc.sysml"}, [][]byte{[]byte(model)})
	exposed := []*symbols.Symbol{lookup(t, idx, "Nested::Outer"), lookup(t, idx, "Nested::Act"),
		lookup(t, idx, "Nested::Param"), lookup(t, idx, "Nested::Shaped"), lookup(t, idx, "Nested::Wheel")}
	rendering, err := r.RenderExposed(exposed, KindMixed, "#mixed")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	wantNoNotices(t, rendering)
	text := rendering.textWith(fullNotation)
	for _, want := range []string{"calc def Nested::Outer\n  out ^result\n  calc Nested::Outer::inner\n    out r\n", "action def Nested::Act\n  action 'step'\n  calc Nested::Act::scale\n    in x\n    out y\n", "part def Nested::Wheel\n",
		"action def Nested::Param (declares no nodes)\n  in f\ncalc def", "calc def Nested::Shaped\n  in g\n  out result\npart def"} {
		if !strings.Contains(text, want) {
			t.Errorf("mixed text lacks %q:\n%s", want, text)
		}
	}
	if n := strings.Count(text, "calc Nested::Outer::inner"); n != 1 {
		t.Errorf("calc inner drawn %d times, want once:\n%s", n, text)
	}
	for _, parameter := range []string{"calc Nested::Param::f", "calc Nested::Shaped::g"} {
		if strings.Contains(text, parameter) {
			t.Errorf("parameter %s drawn as a nested calc beside its pin:\n%s", parameter, text)
		}
	}
	if plain := rendering.Text(); !strings.Contains(plain, "calc def Outer\n  out ^result\n  calc inner\n    out r\n") {
		t.Errorf("default text does not name the nested calc under its owner:\n%s", plain)
	}
}

// A rendering showing no compartments lists no parameters compartment either:
// the pins a form does not draw go back to the not-drawn notice.
func TestHiddenCompartmentsListNoParameters(t *testing.T) {
	rendering := renderSource(t, "Bare::flow", `package Model {
	private import ScalarValues::*;
	calc add { in x : Integer; in y : Integer; return : Integer; x + y }
}
package Bare {
	private import StandardViewDefinitions::*;
	rendering bare :> Views::asInterconnectionDiagram {
		attribute showCompartments : ScalarValues::Boolean default = false;
	}
	view flow : ActionFlowView { expose Model::add; render bare; }
}`)
	mermaid := rendering.Mermaid()
	if strings.Contains(mermaid, "parameters") {
		t.Errorf("Mermaid lists a parameters compartment the rendering hides:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "pin(s) not drawn") || !strings.Contains(mermaid, "add.result") {
		t.Errorf("Mermaid does not notice the pins it neither draws nor lists:\n%s", mermaid)
	}
}

// A calculation yields a result rather than a flow: every form draws the result expression its
// body states in a `result` compartment (§8.2.3.19), and none notes the nodes it does not declare.
func TestCalcShowsItsResultExpressionInEveryForm(t *testing.T) {
	rendering := renderCalc(t, "Calcs::add")
	wantNoNotices(t, rendering)
	root := rendering.Roots[0]
	if len(root.Compartments) != 1 || root.Compartments[0].Name != "result" || len(root.Compartments[0].Rows) != 1 ||
		root.Compartments[0].Rows[0].Text != "x + y" || root.Compartments[0].Rows[0].ID == "" {
		t.Fatalf("add's compartments are %+v, want one `result` row `x + y` with an ID", root.Compartments)
	}
	for form, want := range map[Form]string{
		FormText:     "calc Calcs::add\n  result\n    x + y\n",
		FormDot:      `<b>add</b></td></tr><hr/><tr><td><font point-size="10">result</font></td></tr><tr><td align="left">x + y</td></tr>`,
		FormMermaid:  "**add**\nparameters\nin x\nin y\nout result\nresult\nx + y`",
		FormPlantUML: `**add**\nparameters\nin x\nin y\nout result\nresult\nx + y"`,
		FormD2:       `add\nparameters\nin x\nin y\nout result\nresult\nx + y"`,
		FormDrawio:   `value="result&#xA;x + y"`,
	} {
		written, err := rendering.WriteWith(form, fullNotation)
		if err != nil {
			t.Fatalf("%s: %v", form, err)
		}
		if !strings.Contains(written, want) {
			t.Errorf("%s lacks the result compartment %q:\n%s", form, want, written)
		}
		if strings.Contains(written, "declares no nodes") {
			t.Errorf("%s notes a calculation for declaring no nodes:\n%s", form, written)
		}
	}
}

// The `result` compartment holds the result expression a calculation's own body states: one whose
// result is a bound return parameter, or its type's, has none. Only an action is noted for an empty flow.
func TestResultCompartmentIsTheOwnedResultExpression(t *testing.T) {
	const model = `package Results {
  private import ScalarValues::*;
  calc def Base { in x : Integer; return : Integer; x + 1 }
  calc def Derived :> Base;
  calc def Bound { in x : Integer; return r : Integer = x + 1; }
  calc def Empty;
  calc def Branching {
    in x : Integer;
    return : Integer;
    if x > 0 ? x
       else -x
  }
  calc def Stepped {
    in load : Real;
    action measure { out reading : Real; }
    return : Real;
    measure.reading * load
  }
  calc typed : Base;
  action def Hollow;
}
`
	r, idx := loadSources(t, []string{"results.sysml"}, [][]byte{[]byte(model)})
	for _, tc := range []struct {
		name, result, detail string
		steps                int
	}{
		{"Base", "x + 1", "", 0},
		{"Derived", "", "", 0},
		{"Bound", "", "", 0},
		{"Empty", "", "", 0},
		{"Branching", "if x > 0 ? x else -x", "", 0},
		{"Stepped", "measure.reading * load", "", 1},
		{"typed", "", "", 0},
		{"Hollow", "", "declares no nodes", 0},
	} {
		rendering, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, "Results::"+tc.name)}, KindAction, "#action")
		if err != nil {
			t.Fatalf("render %s: %v", tc.name, err)
		}
		wantNoNotices(t, rendering)
		root := rendering.Roots[0]
		var rows []string
		for _, compartment := range root.Compartments {
			if compartment.Name != "result" {
				t.Errorf("%s has a %q compartment, want none but `result`", tc.name, compartment.Name)
			}
			for _, row := range compartment.Rows {
				rows = append(rows, row.Text)
			}
		}
		if got := strings.Join(rows, "|"); got != tc.result {
			t.Errorf("%s: result rows %q, want %q", tc.name, got, tc.result)
		}
		if root.Detail != tc.detail {
			t.Errorf("%s: detail %q, want %q", tc.name, root.Detail, tc.detail)
		}
		if len(root.Children) != tc.steps {
			t.Errorf("%s: %d steps, want %d", tc.name, len(root.Children), tc.steps)
		}
	}
}

// Graphviz draws a calculation's and an action's parameter pins on the node's border (§8.2.3.19
// param-t, param-b): squares in rows of their own above and below the box, never cells within it.
func TestDOTDrawsParameterPinsOnTheBorder(t *testing.T) {
	for _, fqn := range []string{"Calcs::add", "Calcs::Add", "Calcs::Totaliser::total"} {
		rendering := renderCalc(t, fqn)
		root := rendering.Roots[0]
		for _, options := range []Options{fullNotation, {Show: AllShown(), Ports: PortsFull}, {Show: AllShown(), Palette: PaletteOkabeIto}, {Show: AllShown(), Style: StyleCameo}} {
			dot, err := rendering.WriteWith(FormDot, options)
			if err != nil {
				t.Fatalf("%s: DOT: %v", fqn, err)
			}
			checkDOTSyntax(t, dot)
			line := dotNodeLine(t, dot, root.ID)
			if !strings.Contains(line, `" [shape=plain, style=solid, `) {
				t.Errorf("%s is not the unfilled plain node its box and pins are a label of:\n%s", fqn, line)
			}
			open, shut := strings.Index(line, `<table border="1"`), strings.LastIndex(line, "</table></td></tr>")
			if open < 0 || shut < open {
				t.Fatalf("%s draws no box within its label:\n%s", fqn, line)
			}
			above, box, below := line[:open], line[open:shut], line[shut:]
			if strings.Contains(box, "port=") {
				t.Errorf("%s draws a pin within its box:\n%s", fqn, box)
			}
			for _, port := range root.Ports {
				outside := above
				if port.Direction == PortOut {
					outside = below
				}
				if cell := `<td port="` + port.ID + `" border="1"`; !strings.Contains(outside, cell) {
					t.Errorf("%s: %s pin %s is not a square on its side of the box:\n%s", fqn, port.Direction, port.ID, line)
				}
			}
		}
	}
}

// dotNodeLine is the statement declaring the node of the id in a DOT document.
func dotNodeLine(t *testing.T, dot, id string) string {
	t.Helper()
	for _, line := range strings.Split(dot, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), `"`+id+`" [`) {
			return line
		}
	}
	t.Fatalf("DOT declares no node %s:\n%s", id, dot)
	return ""
}
