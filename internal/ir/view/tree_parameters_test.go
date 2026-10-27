package view

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// renderTreeOf renders one element of calc-parameters.sysml as `#tree:<name>` does.
func renderTreeOf(t *testing.T, fqn string) *Rendering {
	t.Helper()
	r, idx := loadFixture(t, "calc-parameters.sysml")
	rendering, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, fqn)}, KindTree, "#tree")
	if err != nil {
		t.Fatalf("render %s: %v", fqn, err)
	}
	return rendering
}

// A calculation's directed parameters are pins of its tree node, not attribute
// children: `in x`, `in y`, and the anonymous `return` as the `result` output
// it declares (SysML v2 §7.19.2), with no membership edge at any of them.
func TestTreeDrawsCalcParametersAsPins(t *testing.T) {
	rendering := renderTreeOf(t, "ParameterCalcs::add")
	if len(rendering.Roots) != 1 {
		t.Fatalf("roots = %d, want the calc alone", len(rendering.Roots))
	}
	calc := rendering.Roots[0]
	if calc.Kind != "calc" || len(calc.Children) != 0 {
		t.Errorf("root = %s with %d children, want a childless calc", calc.Kind, len(calc.Children))
	}
	wantEqual(t, "pins", pinTexts(calc), []string{"in x", "in y", "out result"})
	for _, port := range calc.Ports {
		if !port.Parameter || port.Type != "Integer" || port.NameSynthesized {
			t.Errorf("pin %s = %+v, want a typed parameter pin the model named", port.Name, port)
		}
	}
	if len(rendering.Edges) != 0 {
		t.Errorf("edges = %v, want none: a parameter is no member node", rendering.Edges)
	}
	wantNoNotices(t, rendering)
}

// The text form lists each parameter under its owner by direction, name and
// type, the default display leaving the type off as it does a member's; the
// pins stay whatever the display shows, as a tree's members do.
func TestTreeTextListsParameterPins(t *testing.T) {
	rendering := renderTreeOf(t, "ParameterCalcs::add")
	if got, want := rendering.Text(), "calc add\n  in x\n  in y\n  out result\n"; !strings.Contains(got, want) {
		t.Errorf("text lacks %q:\n%s", want, got)
	}
	typed, err := rendering.WriteWith(FormText, Options{Show: AllShown()})
	if err != nil {
		t.Fatal(err)
	}
	if want := "calc ParameterCalcs::add\n  in x : Integer\n  in y : Integer\n  out result : Integer\n"; !strings.Contains(typed, want) {
		t.Errorf("typed text lacks %q:\n%s", want, typed)
	}
	data := rendering.DataWith(Options{})
	if len(data.Nodes) != 1 || len(data.Nodes[0].Ports) != 3 {
		t.Errorf("default data = %+v, want the calc with its three pins", data.Nodes)
	}
}

// A calc usage typed by a calc def shows each parameter once: the one it
// redefines as its own, the def's others inherited, the def's anonymous result
// among them; and a calc nested in a part keeps its place under the part.
func TestTreePinsTypedCalcParametersOnce(t *testing.T) {
	rendering := renderTreeOf(t, "ParameterCalcs::Sensor")
	part := rendering.Roots[0]
	var sum, scaled *Node
	for _, child := range part.Children {
		switch child.Name {
		case "sum":
			sum = child
		case "scaled":
			scaled = child
		}
	}
	if sum == nil || scaled == nil {
		t.Fatalf("children of Sensor = %v, want sum and scaled among them", nodeNames(part.Children))
	}
	wantEqual(t, "sum pins", pinTexts(sum), []string{"in x", "in y", "out result"})
	wantEqual(t, "scaled pins", pinTexts(scaled), []string{"in factor", "in x", "inout count", "out doubled", "out r"})
	for i, port := range sum.Ports {
		if inherited := i > 0; port.Inherited != inherited {
			t.Errorf("sum pin %s inherited = %t, want %t", port.Name, port.Inherited, inherited)
		}
	}
	if len(sum.Children)+len(scaled.Children) != 0 {
		t.Errorf("the calcs have %d member nodes, want none", len(sum.Children)+len(scaled.Children))
	}
}

// Action and constraint definitions draw their directed parameters as pins
// too, as the graphical notation sets them (§8.2.3.17, §8.2.3.20); a named
// `return r` is the out pin `r`.
func TestTreePinsActionAndConstraintParameters(t *testing.T) {
	for _, tc := range []struct {
		fqn  string
		pins []string
	}{
		{"ParameterCalcs::Scale", []string{"in x", "in factor", "inout count", "out doubled", "out r"}},
		{"ParameterCalcs::Heat", []string{"in bread", "out toast"}},
		{"ParameterCalcs::Between", []string{"in low", "in value"}},
	} {
		rendering := renderTreeOf(t, tc.fqn)
		root := rendering.Roots[0]
		wantEqual(t, tc.fqn+" pins", pinTexts(root), tc.pins)
		if len(root.Children) != 0 {
			t.Errorf("%s has %d member nodes, want none", tc.fqn, len(root.Children))
		}
	}
}

// A note about a parameter is anchored to the node its pin sits on.
func TestTreeAnchorsParameterNotesToTheOwner(t *testing.T) {
	rendering := renderSource(t, "Diagrams::noted", `package Model {
	private import ScalarValues::*;
	calc def Add {
		in x : Integer;
		return : Integer = x;
	}
}
package Diagrams {
	private import Views::*;
	view noted {
		expose Model::Add;
		render asTreeDiagram;
		metadata DiagramLayout::Note about Model::Add::x { text = "the addend"; x = 200; y = 40; }
	}
}`)
	root := rendering.Roots[0]
	wantEqual(t, "pins", pinTexts(root), []string{"in x", "out result"})
	if len(rendering.Notes) != 1 || rendering.Notes[0].Text != "the addend" || rendering.Notes[0].Anchor != root.ID {
		t.Errorf("notes = %+v, want one note \"the addend\" anchored to %s", rendering.Notes, root.ID)
	}
	if !strings.Contains(rendering.Text(), "the addend") {
		t.Errorf("text form lacks the note:\n%s", rendering.Text())
	}
}

// A view collapsing a calc, or walking no deeper than it, keeps its parameter pins.
func TestTreeKeepsParameterPinsWhenFolded(t *testing.T) {
	rendering := renderSource(t, "Diagrams::folded", `package Model {
	private import ScalarValues::*;
	calc def Add {
		in x : Integer;
		in y : Integer;
		return : Integer = x + y;
		attribute note : String;
	}
}
package Diagrams {
	private import Views::*;
	view folded {
		expose Model::Add;
		render asTreeDiagram;
		metadata DiagramLayout::Layout about Model::Add { x = 0; y = 0; collapsed = true; }
	}
}`)
	root := rendering.Roots[0]
	wantEqual(t, "pins", pinTexts(root), []string{"in x", "in y", "out result"})
	if len(root.Children) != 0 {
		t.Errorf("collapsed calc has %d member nodes, want none", len(root.Children))
	}
}
