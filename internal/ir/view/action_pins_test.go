package view

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// renderActionPins renders one action of action-pins.sysml as `#action:<name>` does.
func renderActionPins(t *testing.T, fqn string) *Rendering {
	t.Helper()
	r, idx := loadFixture(t, "action-pins.sysml")
	rendering, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, fqn)}, KindAction, "#action")
	if err != nil {
		t.Fatalf("render %s: %v", fqn, err)
	}
	return rendering
}

// pinTexts spells a node's pins as `<direction> <name>`, in order.
func pinTexts(node *Node) []string {
	var texts []string
	for _, port := range node.Ports {
		texts = append(texts, port.Direction.String()+" "+port.Name)
	}
	return texts
}

// edgeTexts spells the edges of a kind as `from.pin <arrow> to.pin`, naming
// the nodes and pins rather than their IDs.
func edgeTexts(r *Rendering, kind EdgeKind) []string {
	names := map[string]string{}
	var walk func(*Node)
	walk = func(node *Node) {
		names[node.ID] = node.Name
		for _, port := range node.Ports {
			names[port.ID] = names[node.ID] + "." + port.Name
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range r.Roots {
		walk(root)
	}
	end := func(node, port string) string {
		if port != "" {
			return names[port]
		}
		return names[node]
	}
	var texts []string
	for _, edge := range r.Edges {
		if edge.Kind != kind {
			continue
		}
		texts = append(texts, end(edge.From, edge.FromPort)+" "+edgeArrow(kind)+" "+end(edge.To, edge.ToPort))
	}
	return texts
}

func wantEqual(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s = %q, want %q", what, got, want)
	}
}

func wantNoNotices(t *testing.T, r *Rendering) {
	t.Helper()
	if len(r.Notices) > 0 {
		t.Errorf("notices = %q, want none", r.Notices)
	}
}

// The frame of an action definition carries its own parameters as pins, and
// a nested pin's value naming one (`in b = bread;`) and an explicit bind
// (`bind pack.boxed = toast;`) are binding edges between the frame's pin and
// the node's, the input's into the node and the output's out of it. The
// explicit flow stays a flow.
func TestActionFramePinsDeclared(t *testing.T) {
	rendering := renderActionPins(t, "Pins::ToastBread")
	wantNoNotices(t, rendering)
	root := rendering.Roots[0]
	wantEqual(t, "frame pins", pinTexts(root), []string{"in bread", "out toast"})
	wantEqual(t, "bindings", edgeTexts(rendering, EdgeBinding), []string{"Pins::ToastBread.bread == heat.b", "pack.boxed == Pins::ToastBread.toast"})
	wantEqual(t, "flows", edgeTexts(rendering, EdgeFlow), []string{"heat.t => pack.t"})
	for _, edge := range rendering.Edges {
		if edge.Kind == EdgeBinding && (edge.FromPort == "" || edge.ToPort == "") {
			t.Errorf("binding %s -> %s has no pin at an end", edge.From, edge.To)
		}
	}
}

// A usage takes its frame pins from its type, as a nested node takes its pins
// from the action it performs, and its own nodes bind to them.
func TestActionFramePinsInheritedByUsage(t *testing.T) {
	rendering := renderActionPins(t, "Pins::toaster")
	wantNoNotices(t, rendering)
	wantEqual(t, "frame pins", pinTexts(rendering.Roots[0]), []string{"in bread", "out toast"})
	wantEqual(t, "bindings", edgeTexts(rendering, EdgeBinding), []string{"Pins::toaster.bread == heat2.b"})
}

// The other spellings bind the same way: a qualified value (`in b =
// Wrap::bread;`), a nested pin's value naming another node's pin (`in t =
// heat.t;`, a binding and not a flow), and a redefined output valued by the
// frame's (`out boxed :>> boxed = toast;`).
func TestActionBindingSpellings(t *testing.T) {
	rendering := renderActionPins(t, "Pins::Wrap")
	wantNoNotices(t, rendering)
	wantEqual(t, "bindings", edgeTexts(rendering, EdgeBinding), []string{"Pins::Wrap.bread == heat.b", "heat.t == pack.t", "pack.boxed == Pins::Wrap.toast"})
	wantEqual(t, "flows", edgeTexts(rendering, EdgeFlow), nil)
}

// An explicit bind between two nodes' pins is one binding edge, from the output
// to the input, and no flow.
func TestActionBindingNestedToNested(t *testing.T) {
	rendering := renderActionPins(t, "Pins::Chain")
	wantNoNotices(t, rendering)
	wantEqual(t, "bindings", edgeTexts(rendering, EdgeBinding), []string{"heat.t == pack.t"})
	wantEqual(t, "flows", edgeTexts(rendering, EdgeFlow), nil)
}

// A binding whose other end is no drawn pin — a literal, an attribute — draws
// nothing and reports nothing.
func TestActionBindingToNonPinDrawsNothing(t *testing.T) {
	rendering := renderActionPins(t, "Pins::Loose")
	wantNoNotices(t, rendering)
	wantEqual(t, "frame pins", pinTexts(rendering.Roots[0]), []string{"in bread"})
	wantEqual(t, "bindings", edgeTexts(rendering, EdgeBinding), []string{"Pins::Loose.bread == heat.b"})
}

// An action with no parameters has no frame pins.
func TestActionWithoutParametersHasNoFramePins(t *testing.T) {
	rendering := renderActionPins(t, "Pins::Plain")
	wantNoNotices(t, rendering)
	if pins := rendering.Roots[0].Ports; len(pins) != 0 {
		t.Errorf("frame pins = %q, want none", pinTexts(rendering.Roots[0]))
	}
}

// A nested node with a flow of its own binds its parameter to a node of that
// flow; the edge attaches to the pin of the node standing for it in the frame,
// not to a root the nested rendering discards.
func TestActionBindingWithinNestedFlow(t *testing.T) {
	rendering := renderActionPins(t, "Pins::Stack")
	wantNoNotices(t, rendering)
	wantEqual(t, "bindings", edgeTexts(rendering, EdgeBinding), []string{"outer.x == inner.y", "Pins::Stack.bread == outer.x"})
	wantEdgeEndsDrawn(t, rendering)
}

// A node's own parameter sharing a frame parameter's name, declared by it or
// inherited from its type, shadows the frame's: no frame wire is drawn.
func TestActionBindingShadowedParameterDrawsNothing(t *testing.T) {
	rendering := renderActionPins(t, "Pins::Shadow")
	wantNoNotices(t, rendering)
	wantEqual(t, "frame pins", pinTexts(rendering.Roots[0]), []string{"in x", "in b"})
	wantEqual(t, "bindings", edgeTexts(rendering, EdgeBinding), nil)
}

// A specializing action definition takes its frame pins from its general, as
// a usage does from its type, and its own node binds to one.
func TestActionFramePinsInheritedBySpecialization(t *testing.T) {
	rendering := renderActionPins(t, "Pins::Specialized")
	wantNoNotices(t, rendering)
	wantEqual(t, "frame pins", pinTexts(rendering.Roots[0]), []string{"in bread", "out toast"})
	wantEqual(t, "bindings", edgeTexts(rendering, EdgeBinding), []string{"Pins::Specialized.bread == heat2.b"})
	wantEdgeEndsDrawn(t, rendering)
}

// wantEdgeEndsDrawn checks every edge joins drawn nodes, and its pins drawn
// pins of them.
func wantEdgeEndsDrawn(t *testing.T, r *Rendering) {
	t.Helper()
	ports := map[string]map[string]bool{}
	var walk func(*Node)
	walk = func(node *Node) {
		ports[node.ID] = map[string]bool{}
		for _, port := range node.Ports {
			ports[node.ID][port.ID] = true
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range r.Roots {
		walk(root)
	}
	for _, edge := range r.Edges {
		for _, end := range [][2]string{{edge.From, edge.FromPort}, {edge.To, edge.ToPort}} {
			if _, ok := ports[end[0]]; !ok {
				t.Errorf("edge %s -> %s ends at undrawn node %s", edge.From, edge.To, end[0])
			} else if end[1] != "" && !ports[end[0]][end[1]] {
				t.Errorf("edge %s -> %s ends at undrawn pin %s of %s", edge.From, edge.To, end[1], end[0])
			}
		}
	}
}

// The DOT form draws the frame's pins as the squares the nodes' pins are, inside
// the frame's cluster, and a binding as an undirected edge pin to pin.
func TestDOTActionFramePins(t *testing.T) {
	rendering := renderActionPins(t, "Pins::ToastBread")
	source, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, source)
	for port, label := range map[string]string{`"n0.0"`: `xlabel="bread"`, `"n0.1"`: `xlabel="toast"`} {
		line := dotLine(source, port)
		if line == "" || !strings.Contains(line, label) || !strings.Contains(line, "shape=box") {
			t.Errorf("frame pin %s: %q, want a pin square labelled %s", port, line, label)
		}
	}
	for _, want := range []string{`"n0.0" -> "n1":"n1.0" [arrowhead=none];`, `"n2":"n2.1" -> "n0.1" [arrowhead=none];`} {
		if !strings.Contains(source, want) {
			t.Errorf("DOT lacks %q:\n%s", want, source)
		}
	}
}

// The text form writes an action's pins under their node, the frame's included,
// and names an edge's ends by them.
func TestTextActionFramePins(t *testing.T) {
	rendering := renderActionPins(t, "Pins::ToastBread")
	text := rendering.Text()
	for _, want := range []string{"action def Pins::ToastBread\n  in bread\n  out toast\n", "  action heat : Heat\n    in b\n    out t\n",
		"  Pins::ToastBread.bread == heat.b\n", "  pack.boxed == Pins::ToastBread.toast\n", "  heat.t => pack.t\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
}

// The Mermaid form draws a frame pin an edge ends at as a node inside the
// frame's subgraph, as it draws the nodes' pins, and says which pins it leaves out.
func TestMermaidActionFramePins(t *testing.T) {
	rendering := renderActionPins(t, "Pins::ToastBread")
	source := rendering.Mermaid()
	for _, want := range []string{"\n    n0_p0[\"bread\"]\n", "\n    n0_p1[\"toast\"]\n", `n0_p0 ===|"bread = b"| n1_p0`, `n2_p1 ===|"boxed = toast"| n0_p1`} {
		if !strings.Contains(source, want) {
			t.Errorf("Mermaid lacks %q:\n%s", want, source)
		}
	}
	loose := renderActionPins(t, "Pins::Loose").Mermaid()
	if want := "%% not represented: 3 pin(s) not drawn (heat.t, pack.t, pack.boxed); a flowchart draws the pins an edge ends at"; !strings.Contains(loose, want) {
		t.Errorf("Mermaid lacks the notice %q:\n%s", want, loose)
	}
}

// PlantUML's state grammar has no pin: a binding is an undirected line between
// the frame and the node naming the pins, and the pins left undrawn are noticed.
func TestPlantUMLActionFramePins(t *testing.T) {
	rendering := renderActionPins(t, "Pins::ToastBread")
	source, _ := rendering.PlantUML()
	for _, want := range []string{"n0 -- n1 : bread = b\n", "n2 -- n0 : boxed = toast\n",
		"' not represented: 6 pin(s) not drawn (Pins::ToastBread.bread, Pins::ToastBread.toast, heat.b, heat.t, pack.t, pack.boxed); PlantUML's state grammar has no pin, so the edges name them\n"} {
		if !strings.Contains(source, want) {
			t.Errorf("PlantUML lacks %q:\n%s", want, source)
		}
	}
}
