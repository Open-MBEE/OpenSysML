package view

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// d2GoldenCases are the PlantUML golden models for kinds with a D2 form.
type d2GoldenCase struct {
	name string
	file string
	view string
	kind Kind
}

func d2GoldenCases() []d2GoldenCase {
	var cases []d2GoldenCase
	for _, tc := range plantumlGoldenCases {
		if tc.kind.SupportsForm(FormD2) {
			cases = append(cases, d2GoldenCase{name: tc.name, file: tc.file, view: tc.view, kind: tc.kind})
		}
	}
	return cases
}

// TestGoldenD2 locks the D2 of every kind that has one, and checks each is
// well-formed D2 with the header, the classes it uses and no direction
// statement unasked for.
func TestGoldenD2(t *testing.T) {
	for _, tc := range d2GoldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			rendering := render(t, tc.file, tc.view)
			if rendering.Kind != tc.kind {
				t.Errorf("kind = %q, want %q", rendering.Kind, tc.kind)
			}
			d2, err := rendering.Write(FormD2)
			if err != nil {
				t.Fatalf("write d2: %v", err)
			}
			direct, err := rendering.D2()
			if err != nil || direct != d2 {
				t.Errorf("D2() = %q, %v; want what Write(FormD2) writes", direct, err)
			}
			checkGolden(t, filepath.Join("testdata", tc.name+".d2.golden"), d2)
			checkD2Syntax(t, d2)
			checkD2Renders(t, d2)
			header := fmt.Sprintf("# %s — %s rendering", tc.view, tc.kind)
			if !strings.HasPrefix(d2, header) {
				t.Errorf("D2 does not open with %q:\n%s", header, d2)
			}
			if !rendering.blank() {
				for _, want := range []string{"\nclasses: {\n", `stroke: "#181818"`, "font-size: 14", "\n}\n"} {
					if !strings.Contains(d2, want) {
						t.Errorf("D2 lacks %q:\n%s", want, d2)
					}
				}
			}
			if strings.Contains(d2, "\ndirection: ") {
				t.Errorf("D2 states a direction with none asked for:\n%s", d2)
			}
		})
	}
}

// Every node of the rendering is declared in the D2, a body's start included
// as its initial dot, and every edge is a connection between two paths.
func TestD2DrawsEveryNodeAndEdge(t *testing.T) {
	for _, tc := range d2GoldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			rendering := render(t, tc.file, tc.view)
			d2, err := rendering.D2()
			if err != nil {
				t.Fatalf("D2: %v", err)
			}
			declared := d2Declarations(d2)
			var walk func(node *Node)
			walk = func(node *Node) {
				if !declared[node.ID] {
					t.Errorf("node %s (%s) is not declared:\n%s", node.ID, (labeller{}).head(node), d2)
				}
				for _, child := range node.Children {
					walk(child)
				}
			}
			for _, root := range rendering.Roots {
				walk(root)
			}
			arrows := 0
			for _, line := range strings.Split(d2, "\n") {
				if d2ArrowLine.MatchString(line) {
					arrows++
				}
			}
			containment := 0
			if rendering.Kind == KindTree {
				var count func(node *Node)
				count = func(node *Node) {
					containment += len(node.Children)
					for _, child := range node.Children {
						count(child)
					}
				}
				for _, root := range rendering.Roots {
					count(root)
				}
			}
			if want := len(rendering.Edges) + containment; arrows != want {
				t.Errorf("D2 draws %d connections, the rendering has %d edges:\n%s", arrows, want, d2)
			}
		})
	}
}

// d2Declarations are the keys the D2 declares a shape under, unquoted.
func d2Declarations(d2 string) map[string]bool {
	declared := map[string]bool{}
	for _, line := range strings.Split(d2, "\n") {
		if m := d2DeclarationLine.FindStringSubmatch(line); m != nil {
			declared[strings.Trim(m[2], `"`)] = true
		}
	}
	return declared
}

// The tree draws containment as the Mermaid and DOT trees do: every node at
// the root scope, a line from each node to its children, definitions square
// and usages rounded.
func TestD2TreeIsFlat(t *testing.T) {
	d2, err := render(t, "tree.sysml", "VehicleViews::vehicleView").D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	for _, want := range []string{
		"\nn0: \"«part def»\\nVehicles::Vehicle\" { class: definition }\n",
		"\nn1: \"«part»\\nengine : Engine\" { class: usage }\n",
		"\nn0 -- n1: { class: edge }\n",
		"\n  definition: { style: { fill: white; stroke: \"#181818\"; stroke-width: 1; font-color: black; font-size: 14 } }\n",
		"\n  usage: { style: { fill: white; stroke: \"#181818\"; stroke-width: 1; font-color: black; font-size: 14; border-radius: 8 } }\n",
	} {
		if !strings.Contains(d2, want) {
			t.Errorf("tree D2 lacks %q:\n%s", want, d2)
		}
	}
	for _, line := range strings.Split(d2, "\n") {
		if strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "  ") {
			t.Errorf("tree D2 nests a line: %q", line)
		}
		if d2DeclarationLine.MatchString(line) && strings.HasPrefix(line, " ") && !strings.Contains(d2, "classes: {\n"+line) {
			t.Errorf("tree D2 declares a node inside a container: %q", line)
		}
	}
	if strings.Contains(d2, "->") {
		t.Errorf("tree D2 draws an arrow:\n%s", d2)
	}
}

// The interconnection nests parts as containers, draws the connected ports as
// pins inside them, and joins pins by their paths: a connection a heavy line,
// a flow a dashed arrow.
func TestD2InterconnectionNestsPorts(t *testing.T) {
	rendering := render(t, "interconnection.sysml", "PlantViews::loopView")
	d2, err := rendering.D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	for _, want := range []string{
		"\nn0: \"«part def»\\nLoop\" {\n  class: definition\n  n1: \"«part»\\npump : Pump\" {\n    class: usage\n    \"n1.0\": \"outlet\" { class: pin }\n  }\n",
		"\nn0.n1.\"n1.0\" -- n0.n2.\"n2.0\": \"supply\" { class: connection }\n",
		"\n  connection: { style: { stroke: \"#181818\"; font-size: 13; font-color: black; stroke-width: 3 } }\n",
		"\n  pin: { style: { fill: white; stroke: \"#181818\"; stroke-width: 1; font-color: black; font-size: 10 } }\n",
	} {
		if !strings.Contains(d2, want) {
			t.Errorf("interconnection D2 lacks %q:\n%s", want, d2)
		}
	}
	full, err := rendering.D2With(Options{Ports: PortsFull})
	if err != nil {
		t.Fatalf("D2 full ports: %v", err)
	}
	if !strings.Contains(full, "\"n1.0\": \"«port»\\noutlet : FluidPort\" { class: pin }") {
		t.Errorf("the full display does not name the port's type:\n%s", full)
	}
	flows, err := render(t, "action.sysml", "FlowViews::driveView").D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	if !strings.Contains(flows, "\n  flow: { style: { stroke: \"#181818\"; font-size: 13; font-color: black; stroke-width: 1; stroke-dash: 3 } }\n") ||
		!strings.Contains(flows, `: "torque to reading" { class: flow }`) {
		t.Errorf("a flow is not a dashed arrow:\n%s", flows)
	}
}

// A state diagram nests substates as containers, draws a body's start as an
// initial dot inside it, a region dashed, and transitions by path.
func TestD2StateDiagram(t *testing.T) {
	d2, err := render(t, "state.sysml", "MachineViews::vehicleStates").D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	for _, want := range []string{
		"\nn0: \"«state def»\\nVehicleStates\" {\n  class: definition\n",
		"\n  n2: \"«state»\\noperating\" {\n    class: usage\n",
		"\n  initial: { shape: oval; width: 18; height: 18; style: { fill: black; stroke: black } }\n",
		"\nn0.n1 -> n0.n2: { class: edge }\n",
		"\nn0.n2.n3 -> n0.n2.n4: \"accept Signal [temperature > 0]\" { class: edge }\n",
	} {
		if !strings.Contains(d2, want) {
			t.Errorf("state D2 lacks %q:\n%s", want, d2)
		}
	}
	rendering := render(t, "state.sysml", "MachineViews::vehicleStates")
	starts := 0
	var walk func(node *Node)
	walk = func(node *Node) {
		if node.Kind == startKind {
			starts++
			if !strings.Contains(d2, node.ID+": \"\" { class: initial }") {
				t.Errorf("start %s is not an initial dot:\n%s", node.ID, d2)
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range rendering.Roots {
		walk(root)
	}
	if starts == 0 {
		t.Fatalf("the state rendering has no start node")
	}
}

// An action graph takes the pseudostate glyphs: a fork and a join a bar, a
// decision a diamond named by the node, the final a double-bordered dot; the
// pins an edge ends at are drawn inside their action and joined by path, the
// rest noticed.
func TestD2ActionGlyphsAndPins(t *testing.T) {
	d2, err := render(t, "action.sysml", "FlowViews::driveView").D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	for _, want := range []string{
		"\n  n7: \"\" { class: bar }\n",
		"\n  n8: \"\" { class: bar }\n",
		"\n  n9: \"\" { class: final }\n",
		"\n  n11: \"«decision»\\ncheck\" { class: choice }\n",
		"\n  bar: { shape: rectangle; width: 80; height: 8; style: { fill: black; stroke: black } }\n",
		"\n  choice: { shape: diamond; style: { fill: white; stroke: \"#181818\"; stroke-width: 1; font-color: black; font-size: 14 } }\n",
		"# not represented: 2 fork/join name(s) (split, sync); D2's fork bar draws no label\n",
		"# not represented: 1 pin(s) not drawn (provide.fuel); D2 draws the pins an edge ends at\n",
		"\n  n2: \"«action»\\nprovide : Provide\" {\n    class: usage\n    \"n2.1\": \"torque\" { class: pin }\n  }\n",
		"\n    \"n3.0\": \"reading\" { class: pin }\n",
		"\nn0.n2.\"n2.1\" -> n0.n3.\"n3.0\": \"torque to reading\" { class: flow }\n",
	} {
		if !strings.Contains(d2, want) {
			t.Errorf("action D2 lacks %q:\n%s", want, d2)
		}
	}
	if strings.Contains(d2, "\"n2.0\"") {
		t.Errorf("a pin no edge ends at is drawn:\n%s", d2)
	}
}

// A sequence is D2's sequence diagram: one actor per lifeline in the
// rendering's order, the messages after them in the settled order, a message
// to the sender itself kept, and a blank sequence one actor saying why.
func TestD2SequenceDiagram(t *testing.T) {
	rendering := render(t, "sequence.sysml", "SequenceViews::pubSubView")
	d2, err := rendering.D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	if !strings.Contains(d2, "\nsequence: \"\" {\n  shape: sequence_diagram\n  n0: ") {
		t.Errorf("sequence D2 does not open a sequence diagram:\n%s", d2)
	}
	last := -1
	for _, node := range rendering.Roots {
		at := strings.Index(d2, "\n  "+node.ID+": \"")
		if at < last {
			t.Errorf("lifeline %s is out of order:\n%s", node.ID, d2)
		}
		last = at
	}
	for _, edge := range rendering.Edges {
		want := fmt.Sprintf("\n  %s -> %s: ", edge.From, edge.To)
		if at := strings.Index(d2, want); at < last {
			t.Errorf("message %s -> %s is missing or out of order:\n%s", edge.From, edge.To, d2)
		} else {
			last = at
		}
	}
	for _, line := range strings.Split(d2, "\n") {
		if strings.Contains(line, "--") && !strings.HasPrefix(line, "#") {
			t.Errorf("a message is not an arrow: %q", line)
		}
	}
	empty, err := render(t, "errors.sysml", "ErrorViews::emptySequenceView").D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	if !strings.Contains(empty, "\nsequence: \"\" {\n  shape: sequence_diagram\n  empty: \"the view exposes nothing; the rendering is empty\"\n}\n") || strings.Contains(empty, "classes:") {
		t.Errorf("empty sequence D2:\n%s", empty)
	}
	checkD2Syntax(t, empty)
	checkD2Renders(t, empty)
}

// A GeneralView graph draws its relationships in the class-diagram notation:
// specialization and typing a hollow triangle, composition, reference and
// containment a diamond or circle at the owner, the rest a dashed dependency.
func TestD2GeneralGraphNotation(t *testing.T) {
	definitions, err := render(t, "general.sysml", "GeneralViews::definitionView").D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	packages, err := render(t, "general.sysml", "GeneralViews::packageView").D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	requirements, err := render(t, "general.sysml", "GeneralViews::requirementView").D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	for _, c := range []struct{ d2, want string }{
		{definitions, "\n  specialization: { target-arrowhead: { shape: triangle; style: { filled: false } }; "},
		{definitions, "\n  typing: { target-arrowhead: { shape: triangle; style: { filled: false } }; style: { stroke: \"#181818\"; font-size: 13; font-color: black; stroke-width: 1; stroke-dash: 3 } }\n"},
		{definitions, "\n  composition: { source-arrowhead: { shape: diamond; style: { filled: true } }; "},
		{definitions, "\n  reference: { source-arrowhead: { shape: diamond; style: { filled: false } }; "},
		{definitions, "\nn2 -> n1: { class: specialization }\n"},
		{definitions, "\nn0 <- n9: { class: composition }\n"},
		{packages, "\n  containment: { source-arrowhead: { shape: circle; style: { filled: false } }; "},
		{packages, " <- "},
		{packages, ": \"private import ::*\" { class: dependency }\n"},
		{requirements, ": \"satisfy\" { class: dependency }\n"},
		{requirements, ": \"verify\" { class: dependency }\n"},
	} {
		if !strings.Contains(c.d2, c.want) {
			t.Errorf("general D2 lacks %q:\n%s", c.want, c.d2)
		}
	}
	if _, err := render(t, "case.sysml", "CaseExamples::caseDiagram").D2(); err == nil {
		t.Errorf("a case rendering is written as D2")
	}
}

// The D2 form is offered for graph-shaped kinds and the sequence, including
// GeneralView graphs, but not for tables, matrices, cases or mixed renderings.
func TestD2FormSupport(t *testing.T) {
	for _, kind := range []Kind{KindTree, KindInterconnection, KindState, KindAction, KindSequence, KindRequirement, KindDefinition, KindPackage} {
		if !kind.SupportsForm(FormD2) {
			t.Errorf("%s does not support d2", kind)
		}
	}
	for _, kind := range []Kind{KindCase, KindMixed, KindTable, KindMatrix, KindTextual, KindGeometry} {
		if kind.SupportsForm(FormD2) {
			t.Errorf("%s supports d2", kind)
		}
	}
	if got := fmt.Sprint(KindSequence.SupportedForms()); got != "[text mermaid plantuml d2]" {
		t.Errorf("sequence forms = %s", got)
	}
	if !FormD2.TakesPalette() || FormD2.TakesStyle() {
		t.Errorf("d2 takes a palette and no style: palette %v, style %v", FormD2.TakesPalette(), FormD2.TakesStyle())
	}
	table := render(t, "table.sysml", "TableViews::partsTable")
	_, err := table.D2()
	var wrong *WrongFormError
	if !errors.As(err, &wrong) || wrong.Form != FormD2 || !errors.Is(err, ErrWrongForm) {
		t.Fatalf("D2 of a table: %v", err)
	}
	if !strings.Contains(err.Error(), "a table rendering is not written as d2; ask for text, markdown, csv or tsv") {
		t.Errorf("wrong form error: %v", err)
	}
	if _, err := table.Write(FormD2); !errors.As(err, &wrong) {
		t.Errorf("Write(FormD2) of a table: %v", err)
	}
	for _, tc := range []struct {
		name, file, view string
		kind             Kind
	}{
		{"case", "case.sysml", "CaseExamples::caseDiagram", KindCase},
		{"mixed", "mixed.sysml", "MixedExamples::mixedDiagram", KindMixed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rendering := render(t, tc.file, tc.view)
			_, err := rendering.Write(FormD2)
			if !errors.As(err, &wrong) || wrong.Form != FormD2 || wrong.Kind != tc.kind ||
				wrong.View != tc.view || !errors.Is(err, ErrWrongForm) {
				t.Fatalf("Write(FormD2) of %s: %v", tc.name, err)
			}
			want := fmt.Sprintf("a %s rendering is not written as d2; ask for text, mermaid, dot or plantuml", tc.kind)
			if !strings.Contains(err.Error(), want) {
				t.Errorf("wrong form error = %q, want it to name %q", err, want)
			}
		})
	}
}

// A palette outside the registry and a ports display outside it are refused
// before anything is written.
func TestD2RefusesUnknownOptions(t *testing.T) {
	rendering := render(t, "tree.sysml", "VehicleViews::vehicleView")
	var unknown *UnknownPaletteError
	if _, err := rendering.D2With(Options{Palette: "neon"}); !errors.As(err, &unknown) {
		t.Errorf("unknown palette: %v", err)
	}
	if _, err := rendering.WriteWith(FormD2, Options{Palette: "neon"}); !errors.As(err, &unknown) {
		t.Errorf("unknown palette through Write: %v", err)
	}
	var ports *UnknownPortsError
	if _, err := rendering.D2With(Options{Ports: "some"}); !errors.As(err, &ports) {
		t.Errorf("unknown ports: %v", err)
	}
}

// D2 draws all four directions, so each is stated as asked and none noticed;
// a sequence takes none.
func TestD2Direction(t *testing.T) {
	rendering := render(t, "state.sysml", "MachineViews::vehicleStates")
	for direction, want := range map[Direction]string{
		DirectionTopBottom: "down", DirectionBottomTop: "up", DirectionLeftRight: "right", DirectionRightLeft: "left",
	} {
		d2, err := rendering.D2With(Options{Direction: direction})
		if err != nil {
			t.Fatalf("%s: %v", direction, err)
		}
		if !strings.Contains(d2, "\ndirection: "+want+"\nclasses: {\n") {
			t.Errorf("%s is not stated as %s before the classes:\n%s", direction, want, d2)
		}
		if strings.Contains(d2, "# not represented: direction") {
			t.Errorf("%s is noticed:\n%s", direction, d2)
		}
		checkD2Syntax(t, d2)
	}
	seq, err := render(t, "sequence.sysml", "SequenceViews::pubSubView").D2With(Options{Direction: DirectionLeftRight})
	if err != nil {
		t.Fatalf("sequence: %v", err)
	}
	if strings.Contains(seq, "direction:") {
		t.Errorf("a sequence states a direction:\n%s", seq)
	}
}

// A palette fills the nodes a DOT palette fills, with the same fill per node,
// on every golden model and every palette; the border takes the family colour.
func TestD2PaletteParityWithDOT(t *testing.T) {
	for _, tc := range d2GoldenCases() {
		if tc.kind == KindSequence {
			continue
		}
		rendering := render(t, tc.file, tc.view)
		for _, palette := range Palettes() {
			dot, err := rendering.DOTWith(Options{Palette: palette})
			if err != nil {
				t.Fatalf("%s %s DOT: %v", tc.name, palette, err)
			}
			d2, err := rendering.D2With(Options{Palette: palette})
			if err != nil {
				t.Fatalf("%s %s D2: %v", tc.name, palette, err)
			}
			checkD2Syntax(t, d2)
			dotFills := map[string]string{}
			for _, line := range strings.Split(dot, "\n") {
				if m := dotFillLine.FindStringSubmatch(line); m != nil {
					dotFills[m[1]] = m[2]
				}
			}
			d2Fills := d2Fills(d2)
			if len(dotFills) == 0 {
				t.Errorf("%s %s: DOT fills no node", tc.name, palette)
			}
			if fmt.Sprint(dotFills) != fmt.Sprint(d2Fills) {
				t.Errorf("%s %s: DOT fills %v, D2 %v", tc.name, palette, dotFills, d2Fills)
			}
		}
	}
}

// d2Fills are the fills the D2 gives its shapes, by key: on the declaration
// line of a leaf, on the style line that follows a container's.
func d2Fills(d2 string) map[string]string {
	fills := map[string]string{}
	var open []string
	for _, line := range strings.Split(d2, "\n") {
		if m := d2DeclarationLine.FindStringSubmatch(line); m != nil {
			key := strings.Trim(m[2], `"`)
			if m[4] == " {" {
				open = append(open, key)
			} else if f := d2FillAttr.FindStringSubmatch(m[4]); f != nil {
				fills[key] = f[1]
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "}" && len(open) > 0:
			open = open[:len(open)-1]
		case strings.HasPrefix(trimmed, "style: ") && len(open) > 0:
			if f := d2FillAttr.FindStringSubmatch(trimmed); f != nil {
				fills[open[len(open)-1]] = f[1]
			}
		}
	}
	return fills
}

// The palette golden beside the DOT and PlantUML ones: the interconnection in
// okabe-ito is its black-and-white golden with fills and border colours added.
func TestGoldenD2Palettes(t *testing.T) {
	rendering := render(t, "interconnection.sysml", "PlantViews::loopView")
	d2, err := rendering.WriteWith(FormD2, Options{Palette: PaletteOkabeIto})
	if err != nil {
		t.Fatalf("write d2: %v", err)
	}
	checkGolden(t, filepath.Join("testdata", "interconnection.okabe-ito.d2.golden"), d2)
	checkD2Syntax(t, d2)
	checkD2Renders(t, d2)
	plain, err := rendering.D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	var palette []string
	filled := 0
	for _, line := range strings.Split(d2, "\n") {
		if d2ContainerFillLine.MatchString(line) {
			filled++
			continue
		}
		palette = append(palette, line)
	}
	bw := strings.Split(plain, "\n")
	if len(palette) != len(bw) {
		t.Fatalf("palette D2 has %d lines beside its containers' style lines, black and white %d", len(palette), len(bw))
	}
	for i := range bw {
		if palette[i] == bw[i] {
			continue
		}
		if f := d2FillAttr.FindStringSubmatch(palette[i]); f == nil || !strings.Contains(palette[i], "; stroke: \"#") {
			t.Errorf("line %d differs other than by a fill and a border:\n%s\n%s", i+1, bw[i], palette[i])
		}
		filled++
	}
	if filled == 0 {
		t.Errorf("the palette fills nothing:\n%s", d2)
	}
}

// A label's quote, backslash, line break and substitution opener are
// escaped so D2 shows them as they are; an ID D2 would nest is quoted.
func TestD2EscapesLabels(t *testing.T) {
	rendering := &Rendering{View: "Escapes::view", Kind: KindTree, Roots: []*Node{{
		ID: "n0", Kind: "part def", Name: `say "hi" ${x}` + "\nnext",
		Children: []*Node{{ID: "shape", Kind: "part", Name: "p"}},
	}}}
	d2, err := rendering.D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	for _, want := range []string{
		`n0: "«part def»\nsay \"hi\" \${x}\nnext" { class: definition }`,
		`"shape": "«part»\np" { class: usage }`,
		`n0 -- "shape": { class: edge }`,
	} {
		if !strings.Contains(d2, want) {
			t.Errorf("D2 lacks %q:\n%s", want, d2)
		}
	}
	checkD2Syntax(t, d2)
	checkD2Renders(t, d2)
	if got := d2Quote(`a\b"c` + "\n${d}"); got != `"a\\b\"c\n\${d}"` {
		t.Errorf("d2Quote = %s", got)
	}
}

// The header is the Mermaid one with D2's comment mark, the notices follow it,
// and the geometry is written as comments in the Mermaid shape, between the
// classes and the body.
func TestD2HeaderAndGeometryComments(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	d2, err := rendering.D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	mermaid := rendering.Mermaid()
	for _, line := range strings.Split(mermaid, "\n") {
		if comment, ok := strings.CutPrefix(line, "%% "); ok {
			if !strings.Contains(d2, "\n# "+comment+"\n") && !strings.HasPrefix(d2, "# "+comment+"\n") {
				t.Errorf("D2 lacks the comment %q:\n%s", comment, d2)
			}
		}
	}
	if !strings.Contains(d2, "\n# not represented: 2 positioned node(s) and 1 route(s) kept as comments; D2 pins no position, the dot form does\n") {
		t.Errorf("geometry is not noted as unrepresented:\n%s", d2)
	}
	if plain, _ := render(t, "interconnection.sysml", "PlantViews::loopView").D2(); strings.Contains(plain, "kept as comments") {
		t.Errorf("a rendering without geometry notes it:\n%s", plain)
	}
	if !strings.Contains(d2, "\n}\n# canvas: unit=px w=1200 h=800\n# layout: n1 x=300 y=40 collapsed\n# layout: n2 x=500 y=40 w=120 h=60\n# route: n1->n2 400,70 450,120 500,70\nn0: ") {
		t.Errorf("geometry comments are not between the classes and the body:\n%s", d2)
	}
	anonymous, err := (&Rendering{Kind: KindTree, Notices: []string{"one", "two"}}).D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	if anonymous != "# tree rendering\n# not represented: one\n# not represented: two\nempty: \"the rendering is empty: nothing the view exposes is shown by a tree rendering\"\n" {
		t.Errorf("anonymous D2:\n%s", anonymous)
	}
	checkD2Syntax(t, anonymous)
	checkD2Renders(t, anonymous)
}

// What D2 does not draw is noticed: a drawing style, a font family, notes;
// a Style's colours, size and weight are drawn on nodes and edges.
func TestD2StyleNotices(t *testing.T) {
	rendering := &Rendering{View: "Styled::view", Kind: KindInterconnection, Roots: []*Node{{
		ID: "n0", Kind: "part def", Name: "Box", Children: []*Node{
			{ID: "n1", Kind: "part", Name: "a", Style: &Style{Fill: "FFEE00", Line: "#112233", Text: "#445566", Font: "Serif", FontSize: 6.4, Bold: true}},
			{ID: "n2", Kind: "part", Name: "b", Style: &Style{Italic: true}},
		},
	}}, Edges: []Edge{{From: "n1", To: "n2", Kind: EdgeConnection, Style: &Style{Line: "#ff0000", Fill: "#00ff00", FontSize: 200}}},
		Notes: []Note{{Text: "a note", Anchor: "n1"}}}
	d2, err := rendering.D2With(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	for _, want := range []string{
		"# not represented: style cameo; only the DOT and Mermaid forms draw a diagram in a style\n",
		"# not represented: 1 node(s) and 0 edge(s) name a font; D2 draws no font family, the dot form does\n",
		"# not represented: 1 note(s); the dot form draws notes\n",
		`n1: "«part»\na" { class: usage; style: { fill: "#FFEE00"; stroke: "#112233"; font-color: "#445566"; font-size: 8; bold: true } }`,
		`n2: "«part»\nb" { class: usage; style: { italic: true } }`,
		`n0.n1 -- n0.n2: { class: connection; style: { stroke: "#ff0000"; font-size: 100 } }`,
	} {
		if !strings.Contains(d2, want) {
			t.Errorf("D2 lacks %q:\n%s", want, d2)
		}
	}
	checkD2Syntax(t, d2)
	checkD2Renders(t, d2)
	plain, err := rendering.D2()
	if err != nil {
		t.Fatalf("D2: %v", err)
	}
	if strings.Contains(plain, "style cameo") || strings.Contains(plain, "style pilot") {
		t.Errorf("the default style is noticed:\n%s", plain)
	}
}

var (
	// d2Key matches one key of a path: bare or quoted.
	d2KeyPattern = `(?:[A-Za-z_][A-Za-z0-9_]*|"(?:[^"\\]|\\.)*")`
	// d2DeclarationLine matches a shape declaration: its key, label and either
	// an inline attribute block or an opening brace.
	d2DeclarationLine = regexp.MustCompile(`^(\s*)(` + d2KeyPattern + `): ("(?:[^"\\]|\\.)*")( \{ .* \}| \{)$`)
	// d2ArrowLine matches a connection between two paths with its attributes.
	d2ArrowLine = regexp.MustCompile(`^(\s*)(` + d2KeyPattern + `(?:\.` + d2KeyPattern + `)*) (<-|->|--) (` + d2KeyPattern + `(?:\.` + d2KeyPattern + `)*): (?:"(?:[^"\\]|\\.)*" )?\{ .* \}$`)
	// d2EmptyLine matches the one node a blank rendering shows.
	d2EmptyLine = regexp.MustCompile(`^\s*empty: "(?:[^"\\]|\\.)*"$`)
	// d2ContainerFillLine matches the style line a filled container takes.
	d2ContainerFillLine = regexp.MustCompile(`^\s+style: \{ fill: "#[0-9A-F]{6}"; stroke: "#[0-9A-F]{6}" \}$`)
	// d2FillAttr picks the fill out of a style block.
	d2FillAttr = regexp.MustCompile(`fill: "(#[0-9A-F]{6})"`)
	// d2ClassLine matches one class declaration inside the classes block.
	d2ClassLine = regexp.MustCompile(`^  [a-z]+: \{ .* \}$`)
)

// checkD2Syntax checks the shape of a D2 file: braces balance, every quoted
// string closes on its line, every statement is one the writer emits, and
// every path a connection names is declared in the scope it is written in.
// It is a checker, not a parser.
func checkD2Syntax(t *testing.T, d2 string) {
	t.Helper()
	if !strings.HasSuffix(d2, "\n") {
		t.Fatalf("D2 does not end in a newline")
	}
	lines := strings.Split(strings.TrimSuffix(d2, "\n"), "\n")
	if !strings.HasPrefix(lines[0], "# ") {
		t.Fatalf("D2 does not open with a comment:\n%s", d2)
	}
	declared := map[string]bool{}
	type arrow struct{ from, to string }
	var arrows []arrow
	var scope []string
	classes := false
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Count(strings.ReplaceAll(line, `\"`, ""), `"`)%2 != 0 {
			t.Fatalf("line %d leaves a quote open: %q", i+1, line)
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case line == "classes: {":
			classes = true
			continue
		case classes:
			if trimmed == "}" {
				classes = false
			} else if !d2ClassLine.MatchString(line) {
				t.Fatalf("line %d is no class declaration: %q", i+1, line)
			}
			continue
		case line == "direction: down", line == "direction: up", line == "direction: left", line == "direction: right":
			continue
		case line == `sequence: "" {`:
			scope = append(scope, "sequence")
			continue
		case d2EmptyLine.MatchString(line):
			continue
		case trimmed == "shape: sequence_diagram", strings.HasPrefix(trimmed, "class: "), strings.HasPrefix(trimmed, "style: { "):
			if len(scope) == 0 {
				t.Fatalf("line %d sets an attribute at the root: %q", i+1, line)
			}
			continue
		case trimmed == "}":
			if len(scope) == 0 {
				t.Fatalf("line %d closes more braces than opened:\n%s", i+1, d2)
			}
			scope = scope[:len(scope)-1]
			continue
		}
		if m := d2DeclarationLine.FindStringSubmatch(line); m != nil {
			if len(m[1]) != 2*len(scope) {
				t.Errorf("line %d is indented %d deep in scope %v: %q", i+1, len(m[1]), scope, line)
			}
			declared[strings.Join(append(append([]string(nil), scope...), m[2]), ".")] = true
			if m[4] == " {" {
				scope = append(scope, m[2])
			}
			continue
		}
		if m := d2ArrowLine.FindStringSubmatch(line); m != nil {
			prefix := strings.Join(scope, ".")
			if prefix != "" {
				prefix += "."
			}
			arrows = append(arrows, arrow{prefix + m[2], prefix + m[4]})
			continue
		}
		t.Fatalf("line %d is no statement the writer emits: %q\n%s", i+1, line, d2)
	}
	if classes {
		t.Fatalf("classes is never closed:\n%s", d2)
	}
	if len(scope) != 0 {
		t.Fatalf("braces do not balance (%v open):\n%s", scope, d2)
	}
	for _, a := range arrows {
		for _, end := range []string{a.from, a.to} {
			if !declared[end] {
				t.Errorf("connection %s -> %s names the undeclared path %s:\n%s", a.from, a.to, end, d2)
			}
		}
	}
}

// checkD2Renders asks a d2 executable to compile the diagram when one is at
// hand, at the path OPENSYSML_D2 names or on the PATH, and is silent
// otherwise: nothing here depends on it.
func checkD2Renders(t *testing.T, d2 string) {
	t.Helper()
	path := os.Getenv("OPENSYSML_D2")
	if path == "" {
		found, err := exec.LookPath("d2")
		if err != nil {
			return
		}
		path = found
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "diagram.d2")
	if err := os.WriteFile(source, []byte(d2), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, source, filepath.Join(dir, "diagram.svg"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("d2 rejects the diagram: %v\n%s\n%s", err, out, d2)
	}
}
