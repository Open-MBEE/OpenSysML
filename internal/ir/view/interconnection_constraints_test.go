package view

import (
	"path/filepath"
	"strings"
	"testing"
)

// A constraint usage is a node of an interconnection rendering, nested under
// its owner as a part is, with the parameters of its def and those it declares
// itself as pins on its border; a binding between a parameter and an attribute
// of the owner, or between two parameters, ends at the pins. No constraint is
// reported as having no place in the rendering.
func TestInterconnectionDrawsConstraintUsagesWithTheirParameters(t *testing.T) {
	rendering := render(t, "interconnection-constraints.sysml", "ParametricViews::amplifierView")
	byName := nodesByName(rendering)
	amplify, mix := byName["amplify"], byName["mix"]
	if amplify == nil || mix == nil || amplify.Kind != "constraint" || mix.Kind != "constraint" {
		t.Fatalf("constraint nodes: amplify = %+v, mix = %+v", amplify, mix)
	}
	if amplifier := byName["Parametrics::Amplifier"]; amplifier == nil || len(amplifier.Children) != 6 {
		t.Errorf("Amplifier = %+v, want the four attributes and two constraints nested under it", amplifier)
	}
	wantPins := map[string][]string{"amplify": {"in input", "in gain", "out output"}, "mix": {"in a", "in b", "out total", "in weight"}}
	for name, want := range wantPins {
		var got []string
		for _, port := range byName[name].Ports {
			if port.Parameter {
				t.Errorf("%s.%s is a parameter symbol; a constraint parameter is drawn with the ports", name, port.Name)
			}
			got = append(got, port.Direction.String()+" "+port.Name)
		}
		if strings.Join(got, ", ") != strings.Join(want, ", ") {
			t.Errorf("%s pins = %v, want %v", name, got, want)
		}
	}
	pinOf := func(node *Node, name string) string {
		for _, port := range node.Ports {
			if port.Name == name {
				return port.ID
			}
		}
		return ""
	}
	type end struct{ node, port string }
	wantEdges := map[string][2]end{
		"signalIn":  {{byName["signal"].ID, ""}, {amplify.ID, pinOf(amplify, "input")}},
		"gainIn":    {{byName["gain"].ID, ""}, {amplify.ID, pinOf(amplify, "gain")}},
		"amplified": {{amplify.ID, pinOf(amplify, "output")}, {mix.ID, pinOf(mix, "a")}},
		"noiseIn":   {{byName["noise"].ID, ""}, {mix.ID, pinOf(mix, "b")}},
		"mixed":     {{mix.ID, pinOf(mix, "total")}, {byName["output"].ID, ""}},
	}
	for _, edge := range rendering.Edges {
		want, ok := wantEdges[edge.Label]
		if !ok {
			t.Errorf("unexpected edge %+v", edge)
			continue
		}
		delete(wantEdges, edge.Label)
		if edge.Kind != EdgeBinding || edge.From != want[0].node || edge.FromPort != want[0].port || edge.To != want[1].node || edge.ToPort != want[1].port {
			t.Errorf("%s = %+v, want a binding from %v to %v", edge.Label, edge, want[0], want[1])
		}
	}
	for name := range wantEdges {
		t.Errorf("binding %s is not drawn", name)
	}
	for _, notice := range rendering.Notices {
		if strings.Contains(notice, "no place") {
			t.Errorf("a constraint is reported as having no place: %s", notice)
		}
	}
}

// Without port symbols a constraint's parameter squares are left out with the
// ports', and a binding that ended at one ends at the constraint itself, in
// every form; shown again, the squares are back.
func TestPortSymbolsLeftOutHideConstraintParameters(t *testing.T) {
	rendering := render(t, "interconnection-constraints.sysml", "ParametricViews::amplifierView")
	rendering.Shown = Shown{}
	shown := rendering.showing(nil)
	byName := nodesByName(shown)
	for _, name := range []string{"amplify", "mix"} {
		if node := byName[name]; node == nil || len(node.Ports) != 0 {
			t.Errorf("%s = %+v, want a constraint with no pin", name, node)
		}
	}
	for _, edge := range shown.Edges {
		if edge.FromPort != "" || edge.ToPort != "" {
			t.Errorf("%s still ends at a pin: %+v", edge.Label, edge)
		}
		if edge.Label == "amplified" && (edge.From != byName["amplify"].ID || edge.To != byName["mix"].ID) {
			t.Errorf("amplified = %+v, want it from amplify to mix", edge)
		}
	}
	for _, form := range []Form{FormText, FormDot, FormMermaid, FormPlantUML, FormD2} {
		out, err := rendering.WriteWith(form, Options{Ports: PortsFull})
		if err != nil {
			t.Fatalf("%s: %v", form, err)
		}
		for _, pin := range []string{"input", "total", "weight"} {
			if strings.Contains(out, pin) {
				t.Errorf("%s draws parameter %s without port symbols:\n%s", form, pin, out)
			}
		}
		out, err = rendering.WriteWith(form, Options{Ports: PortsFull, Show: Shown{ElementPortSymbols: true}})
		if err != nil {
			t.Fatalf("%s: %v", form, err)
		}
		for _, pin := range []string{"input", "total", "weight"} {
			if !strings.Contains(out, pin) {
				t.Errorf("%s leaves parameter %s out with port symbols shown:\n%s", form, pin, out)
			}
		}
	}
}

// A view exposing a constraint usage's def parameters beside it, as a migrated
// parametric diagram does, draws them once, as the usage's pins: a Layout about
// a parameter sets its square on the nearest side of the usage's border, level
// with the Layout, and a binding's route ends on the square.
func TestDOTPlacesConstraintParametersWhereTheirLayoutsSay(t *testing.T) {
	rendering := render(t, "interconnection-constraints.sysml", "ParametricViews::placedView")
	byName := nodesByName(rendering)
	var roots []string
	for _, node := range rendering.Roots {
		roots = append(roots, node.Name)
	}
	if strings.Join(roots, ", ") != "Parametrics::Amplifier::signal, Parametrics::Amplifier::gain, Parametrics::Amplifier::noise, Parametrics::Amplifier::output, Parametrics::Amplifier::amplify, Parametrics::Amplifier::mix" {
		t.Errorf("roots = %v, want the four attributes and two constraints, the exposed parameters drawn as pins alone", roots)
	}
	for _, port := range byName["Parametrics::Amplifier::amplify"].Ports {
		if port.Geometry == nil || !port.Geometry.HasSize || port.Geometry.Width != 15 {
			t.Errorf("amplify.%s geometry = %+v, want its 15x15 Layout", port.Name, port.Geometry)
		}
	}
	dot, err := rendering.DOTWith(fullNotation)
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkGolden(t, filepath.Join("testdata", "interconnection-constraints-placed.dot.golden"), dot)
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		// input's Layout is (200, 45) 15x15 on amplify's left border at x=200: its
		// square sits outside that side, level with the Layout's centre, y up.
		`xlabel="^input", fontsize=8, width=0.16666666666666666, height=0.16666666666666666, fixedsize=true, pos="194,167.5!", pin=true];`,
		// output's Layout is at the right border: outside it, at the Layout's centre.
		`xlabel="^output", fontsize=8, width=0.16666666666666666, height=0.16666666666666666, fixedsize=true, pos="406,152.5!", pin=true];`,
		// The binding's route ends on the square, not where the Route point stopped.
		`"n0" -> "n4.0" [label="signalIn", arrowhead=none, pos="120,165 120,165 200,168 200,168", lp="159.5,179.5"];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
}

// Two usages of one constraint def each draw the def's parameter as their own:
// a Layout about `leftGain::input` places leftGain's square and one about
// `rightGain::input` rightGain's, as a Layout about an inherited state is the
// usage's; the DOT sets each square level with its own Layout.
func TestConstraintParameterLayoutsResolvePerUsage(t *testing.T) {
	rendering := render(t, "interconnection-constraints.sysml", "ParametricViews::stereoView")
	byName := nodesByName(rendering)
	wantY := map[string]float64{"leftGain": 30, "rightGain": 160}
	for name, y := range wantY {
		node := byName[name]
		if node == nil {
			t.Fatalf("%s is not drawn", name)
		}
		for _, port := range node.Ports {
			if port.Name == "input" && (port.Geometry == nil || port.Geometry.Y != y) {
				t.Errorf("%s.input geometry = %+v, want its own Layout at y=%v", name, port.Geometry, y)
			}
		}
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{`xlabel="^input", fontsize=8, width=0.16666666666666666, height=0.16666666666666666, fixedsize=true, pos="194,`} {
		if strings.Count(dot, want) != 2 {
			t.Errorf("DOT does not place both input squares on the left border:\n%s", dot)
		}
	}
	clone := rendering.Clone()
	nodesByName(clone)["leftGain"].Ports[0].Geometry.Y = -1
	if byName["leftGain"].Ports[0].Geometry.Y == -1 {
		t.Errorf("the clone shares pin geometry with the original")
	}
}

// A parameter's Layout places its square only on a node a Layout places too:
// where the node has no place of its own, the square is placed as a routed or
// free pin is, and the rendering is positioned by nothing but the pins.
func TestPinLayoutAloneLeavesTheNodeUnplaced(t *testing.T) {
	rendering := render(t, "interconnection-constraints.sysml", "ParametricViews::amplifierView")
	byName := nodesByName(rendering)
	byName["amplify"].Ports[0].Geometry = &Geometry{X: 200, Y: 45, Width: 15, Height: 15, HasSize: true}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if !strings.Contains(dot, "// layout: dot\n") || strings.Contains(dot, "pin=true") {
		t.Errorf("a pin's Layout alone positions the rendering:\n%s", dot)
	}
}

// A mixed rendering draws a constraint usage as an interconnection one does,
// with its parameters as pins and the bindings ending at them.
func TestMixedDrawsConstraintUsagesWithTheirParameters(t *testing.T) {
	rendering := render(t, "interconnection-constraints.sysml", "ParametricViews::mixedView")
	if rendering.Kind != KindMixed {
		t.Fatalf("kind = %q, want mixed", rendering.Kind)
	}
	byName := nodesByName(rendering)
	amplify, mix := byName["amplify"], byName["mix"]
	if amplify == nil || mix == nil || len(amplify.Ports) != 3 || len(mix.Ports) != 4 {
		t.Fatalf("constraint nodes: amplify = %+v, mix = %+v, want their parameters as pins", amplify, mix)
	}
	bindings := 0
	for _, edge := range rendering.Edges {
		if edge.Kind != EdgeBinding {
			continue
		}
		bindings++
		if edge.FromPort == "" && edge.ToPort == "" {
			t.Errorf("%s ends at no pin: %+v", edge.Label, edge)
		}
	}
	if bindings != 5 {
		t.Errorf("%d bindings drawn, want 5", bindings)
	}
	for _, notice := range rendering.Notices {
		if strings.Contains(notice, "no place") {
			t.Errorf("a constraint is reported as having no place: %s", notice)
		}
	}
}
