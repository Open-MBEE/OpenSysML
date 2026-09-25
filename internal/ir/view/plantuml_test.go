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

// plantumlGoldenCases are the models behind every Mermaid golden a PlantUML
// form exists for: each has a `.plantuml.golden` beside its `.mermaid.golden`.
var plantumlGoldenCases = []struct {
	name string
	file string
	view string
	kind Kind
}{
	{"tree", "tree.sysml", "VehicleViews::vehicleView", KindTree},
	{"interconnection", "interconnection.sysml", "PlantViews::loopView", KindInterconnection},
	{"layout", "layout.sysml", "PlantViews::placedView", KindInterconnection},
	{"state", "state.sysml", "MachineViews::vehicleStates", KindState},
	{"state-entry", "state-entry.sysml", "MachineViews::thermostat", KindState},
	{"action", "action.sysml", "FlowViews::driveView", KindAction},
	{"typed-action", "typed-behavior.sysml", "TypedViews::cycleView", KindAction},
	{"typed-state", "typed-behavior.sysml", "TypedViews::boilerView", KindState},
	{"filters", "filters.sysml", "FilteredViews::safetyView", KindTree},
	{"sequence", "sequence.sysml", "SequenceViews::pubSubView", KindSequence},
	{"sequence-vehicle", "sequence-vehicle.sysml", "VehicleSequenceViews::startVehicleView", KindSequence},
	{"sequence-notices", "sequence-notices.sysml", "NoticeViews::handshakeView", KindSequence},
	{"sequence-order", "sequence-order.sysml", "OrderingViews::relayView", KindSequence},
	{"sequence-cycle", "sequence-order.sysml", "OrderingViews::deadlockView", KindSequence},
	{"sequence-empty", "errors.sysml", "ErrorViews::emptySequenceView", KindSequence},
}

// TestGoldenPlantUML locks the PlantUML of every kind that has one, from the
// models the Mermaid goldens use, and checks each is well-formed PlantUML with
// the header, the style block and no direction statement unasked for.
func TestGoldenPlantUML(t *testing.T) {
	for _, tc := range plantumlGoldenCases {
		t.Run(tc.name, func(t *testing.T) {
			rendering := render(t, tc.file, tc.view)
			if rendering.Kind != tc.kind {
				t.Errorf("kind = %q, want %q", rendering.Kind, tc.kind)
			}
			puml, err := rendering.Write(FormPlantUML)
			if err != nil {
				t.Fatalf("write plantuml: %v", err)
			}
			direct, err := rendering.PlantUML()
			if err != nil || direct != puml {
				t.Errorf("PlantUML() = %q, %v; want what Write(FormPlantUML) writes", direct, err)
			}
			checkGolden(t, filepath.Join("testdata", tc.name+".plantuml.golden"), puml)
			checkPlantUMLSyntax(t, puml)
			checkPlantUMLRenders(t, puml)
			header := fmt.Sprintf("@startuml\n' %s — %s rendering", tc.view, tc.kind)
			if !strings.HasPrefix(puml, header) {
				t.Errorf("PlantUML does not open with %q:\n%s", header, puml)
			}
			for _, want := range []string{"<style>\n", "\n</style>\n", "\n  FontName SansSerif\n", "\n  LineColor #181818\n", "\n  Shadowing 0.0\n",
				"\n.usage {\n  RoundCorner 20\n}\n", "\nskinparam wrapWidth 300\n", "\nhide stereotype\n"} {
				if !strings.Contains(puml, want) {
					t.Errorf("PlantUML lacks %q:\n%s", want, puml)
				}
			}
			if strings.Contains(puml, " direction\n") {
				t.Errorf("PlantUML states a direction with none asked for:\n%s", puml)
			}
		})
	}
}

// The Mermaid and PlantUML forms show the same nodes: every node ID of the
// rendering is an alias in the PlantUML, and every edge is an arrow between two.
func TestPlantUMLDrawsEveryNodeAndEdge(t *testing.T) {
	for _, tc := range plantumlGoldenCases {
		t.Run(tc.name, func(t *testing.T) {
			rendering := render(t, tc.file, tc.view)
			puml, err := rendering.PlantUML()
			if err != nil {
				t.Fatalf("PlantUML: %v", err)
			}
			var walk func(node *Node)
			walk = func(node *Node) {
				if node.Kind != startKind && !strings.Contains(puml, " as "+node.ID+"\n") && !strings.Contains(puml, " as "+node.ID+" ") {
					t.Errorf("node %s (%s) is not declared:\n%s", node.ID, (labeller{}).head(node), puml)
				}
				for _, child := range node.Children {
					walk(child)
				}
			}
			for _, root := range rendering.Roots {
				walk(root)
			}
			arrows := 0
			for _, line := range strings.Split(puml, "\n") {
				if plantumlArrowLine.MatchString(line) {
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
				t.Errorf("PlantUML draws %d arrows, the rendering has %d edges:\n%s", arrows, want, puml)
			}
		})
	}
}

// The tree draws containment as the Mermaid tree does: an edge from a node to
// each child, every node a class, and hides the circle and empty compartments.
func TestPlantUMLTreeIsAClassDiagram(t *testing.T) {
	puml, err := render(t, "tree.sysml", "VehicleViews::vehicleView").PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	for _, want := range []string{
		"\nhide circle\nhide empty members\n",
		"class \"**Vehicles::Vehicle**\\n<size:10>//«part def»//</size>\" as n0 <<part def>>\n",
		"class \"**engine : Engine**\\n<size:10>//«part»//</size>\" as n1 <<part>> <<usage>>\nn0 -- n1\n",
		"class \"**connect**\" as n3 <<connect>> <<usage>>\nn0 -- n3\n",
	} {
		if !strings.Contains(puml, want) {
			t.Errorf("tree PlantUML lacks %q:\n%s", want, puml)
		}
	}
}

// The interconnection nests rectangles: a node with children is a block, a
// connection an undirected heavy line, a flow a dashed arrow.
func TestPlantUMLInterconnectionNestsRectangles(t *testing.T) {
	puml, err := render(t, "interconnection.sysml", "PlantViews::loopView").PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	for _, want := range []string{
		"rectangle \"**Loop**\\n<size:10>//«part def»//</size>\" as n0 <<part def>> {\n  rectangle \"**pump : Pump**\\n<size:10>//«part»//</size>\" as n1 <<part>> <<usage>>\n",
		"\n}\nn1 -[thickness=3]- n2 : supply\nn1 -[dashed]-> n2 : of Water\n@enduml\n",
	} {
		if !strings.Contains(puml, want) {
			t.Errorf("interconnection PlantUML lacks %q:\n%s", want, puml)
		}
	}
}

// A state rendering is a state diagram: composite states are blocks, a body's
// start is the `[*]` marker inside it with its entry transitions, and every
// other transition follows the states. Regions carry their stereotype for the
// dashed border rule.
func TestPlantUMLStateDiagram(t *testing.T) {
	puml, err := render(t, "state-entry.sysml", "MachineViews::thermostat").PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	mermaid := render(t, "state-entry.sysml", "MachineViews::thermostat").Mermaid()
	for _, want := range []string{"\nhide empty description\n", "<<state def>> {\n", "<<region>> {\n", "  [*] --> ", "<<state>> <<usage>>"} {
		if !strings.Contains(puml, want) {
			t.Errorf("state PlantUML lacks %q:\n%s", want, puml)
		}
	}
	// Each `[*]` transition of the Mermaid form is one of the PlantUML form.
	if got, want := strings.Count(puml, "[*] --> "), strings.Count(mermaid, "[*] --> "); got != want {
		t.Errorf("PlantUML has %d start transitions, Mermaid %d", got, want)
	}
	if strings.Contains(puml, "<<start>>") || strings.Contains(puml, " as n") && strings.Contains(puml, "\"\" as") {
		t.Errorf("PlantUML declares a start node:\n%s", puml)
	}
}

// An action rendering takes the state grammar: control nodes are states with
// their kind as stereotype, successions solid arrows and flows dashed and
// labelled, every node and edge of the graph drawn. The `start` and `done` the
// language names head as their kind, since those names are not the body's.
func TestPlantUMLActionUsesStateGrammar(t *testing.T) {
	puml, err := render(t, "action.sysml", "FlowViews::driveView").PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	for _, want := range []string{
		"\nhide empty description\n",
		"state \"**Drive**\\n<size:10>//«action def»//</size>\" as n0 <<action def>> {\n",
		"  state \"**initial**\" as n1 <<start>>\n",
		"  state \"**split**\\n<size:10>//«fork»//</size>\" as n7 <<fork>>\n",
		"  state \"**final**\" as n9 <<end>>\n",
		"  state \"**check**\\n<size:10>//«decision»//</size>\" as n11 <<choice>>\n",
		"  state \"**monitor**\\n<size:10>//«action»//</size>\\nown flow\" as n3 <<action>> <<usage>> {\n",
		"\nn2 -[dashed]-> n3 : torque to reading\n",
		"\nn11 --> n9 : [speed <U+003E> 0]\n",
		"\nn1 --> n7\n",
	} {
		if !strings.Contains(puml, want) {
			t.Errorf("action PlantUML lacks %q:\n%s", want, puml)
		}
	}
	if strings.Contains(puml, "\nstart\n") || strings.Contains(puml, ":") && strings.Contains(puml, ";\n") {
		t.Errorf("action PlantUML uses activity grammar:\n%s", puml)
	}
}

// A sequence rendering is a sequence diagram: participants in root order,
// messages in edge order, an unlabelled message without the colon, and an
// empty rendering one participant carrying the reason.
func TestPlantUMLSequenceDiagram(t *testing.T) {
	puml, err := render(t, "sequence-notices.sysml", "NoticeViews::handshakeView").PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	for _, want := range []string{
		"' not represented: message pending states no source and target; no message is drawn\n",
		"participant \"**caller**\\n<size:10>//«part»//</size>\" as n0 <<part>> <<usage>>\nparticipant \"**callee**\\n<size:10>//«part»//</size>\" as n1 <<part>> <<usage>>\nn0 -> n0 : echo\nn0 -> n1 : of Ping\n@enduml\n",
	} {
		if !strings.Contains(puml, want) {
			t.Errorf("sequence PlantUML lacks %q:\n%s", want, puml)
		}
	}
	empty, err := render(t, "errors.sysml", "ErrorViews::emptySequenceView").PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	if !strings.Contains(empty, "\nparticipant \"the view exposes nothing; the rendering is empty\" as empty\n@enduml\n") {
		t.Errorf("empty sequence PlantUML:\n%s", empty)
	}
	rendering := &Rendering{Kind: KindSequence, Roots: []*Node{{ID: "a", Kind: "part", Name: "a"}, {ID: "b", Kind: "part", Name: "b"}},
		Edges: []Edge{{From: "a", To: "b", Kind: EdgeFlow}, {From: "b", To: "a", Label: "ack", Kind: EdgeFlow}}}
	puml, err = rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	if !strings.Contains(puml, "\na -> b\nb -> a : ack\n") {
		t.Errorf("unlabelled message:\n%s", puml)
	}
	if puml, _ := rendering.PlantUMLWith(Options{Direction: DirectionLeftRight}); strings.Contains(puml, "direction") {
		t.Errorf("sequence PlantUML states a direction:\n%s", puml)
	}
}

// Every kind drawn as a graph, and a sequence, has the PlantUML form; a table,
// a textual and a geometry rendering have none, and asking is a typed error.
func TestPlantUMLFormSupport(t *testing.T) {
	for _, kind := range Kinds() {
		want := kind == KindTree || kind == KindInterconnection || kind == KindState || kind == KindAction || kind == KindSequence
		if got := kind.SupportsForm(FormPlantUML); got != want {
			t.Errorf("%s.SupportsForm(plantuml) = %v, want %v", kind, got, want)
		}
		if got := kind.SupportsPalette(); got != (kind.SupportsForm(FormDot) || kind.SupportsForm(FormPlantUML)) {
			t.Errorf("%s.SupportsPalette() = %v", kind, got)
		}
		if kind.MachineForm() == FormPlantUML {
			t.Errorf("%s has PlantUML as its machine form", kind)
		}
	}
	if got := fmt.Sprint(Forms()); got != "[text mermaid markdown dot plantuml]" {
		t.Errorf("Forms() = %s", got)
	}
	if got := fmt.Sprint(DiagramForms()); got != "[mermaid dot plantuml]" {
		t.Errorf("DiagramForms() = %s", got)
	}
	if got := KindSequence.SupportedForms(); fmt.Sprint(got) != "[text mermaid plantuml]" {
		t.Errorf("sequence forms = %v", got)
	}
	if !FormPlantUML.TakesPalette() || !FormDot.TakesPalette() || FormMermaid.TakesPalette() {
		t.Error("TakesPalette: want dot and plantuml alone")
	}
	unsupported := []*Rendering{
		render(t, "table.sysml", "TableViews::partsTable"),
		{Kind: KindTextual, View: "T::textual"},
		{Kind: KindGeometry, View: "T::geometry"},
	}
	for _, rendering := range unsupported {
		t.Run(string(rendering.Kind), func(t *testing.T) {
			for name, write := range map[string]func() (string, error){
				"Write":    func() (string, error) { return rendering.Write(FormPlantUML) },
				"PlantUML": rendering.PlantUML,
			} {
				out, err := write()
				var wrong *WrongFormError
				if !errors.As(err, &wrong) || !errors.Is(err, ErrWrongForm) {
					t.Fatalf("%s error = %v, want a *WrongFormError", name, err)
				}
				if out != "" {
					t.Errorf("%s wrote %q alongside the error", name, out)
				}
				if wrong.Form != FormPlantUML || wrong.Kind != rendering.Kind {
					t.Errorf("%s error = %+v, want form plantuml of kind %s", name, wrong, rendering.Kind)
				}
			}
		})
	}
}

// An unregistered palette is refused before anything is written, as DOT refuses it.
func TestPlantUMLRefusesAnUnregisteredPalette(t *testing.T) {
	rendering := render(t, "tree.sysml", "VehicleViews::vehicleView")
	for name, write := range map[string]func() (string, error){
		"PlantUMLWith": func() (string, error) { return rendering.PlantUMLWith(Options{Palette: "pastel"}) },
		"WriteWith":    func() (string, error) { return rendering.WriteWith(FormPlantUML, Options{Palette: "pastel"}) },
	} {
		out, err := write()
		var unknown *UnknownPaletteError
		if !errors.As(err, &unknown) || unknown.Name != "pastel" {
			t.Errorf("%s error = %v, want an *UnknownPaletteError for pastel", name, err)
		}
		if out != "" {
			t.Errorf("%s wrote %q alongside the error", name, out)
		}
	}
}

// Direction is stated as PlantUML states it; a reversed direction takes its
// nearest and is noted as not represented; the empty one states nothing.
func TestPlantUMLDirection(t *testing.T) {
	rendering := render(t, "state.sysml", "MachineViews::vehicleStates")
	cases := []struct {
		direction Direction
		statement string
		notice    bool
	}{
		{"", "", false},
		{DirectionTopBottom, "top to bottom direction", false},
		{DirectionLeftRight, "left to right direction", false},
		{DirectionBottomTop, "top to bottom direction", true},
		{DirectionRightLeft, "left to right direction", true},
	}
	for _, tc := range cases {
		puml, err := rendering.PlantUMLWith(Options{Direction: tc.direction})
		if err != nil {
			t.Fatalf("%s: %v", tc.direction, err)
		}
		statements := strings.Count(puml, " direction\n")
		if tc.statement == "" && statements != 0 {
			t.Errorf("%q states a direction:\n%s", tc.direction, puml)
		}
		if tc.statement != "" && (statements != 1 || !strings.Contains(puml, "</style>\nskinparam wrapWidth 300\nhide stereotype\n"+tc.statement+"\n")) {
			t.Errorf("%q: want %q once after the style:\n%s", tc.direction, tc.statement, puml)
		}
		notice := "' not represented: direction " + string(tc.direction) + "; PlantUML draws no reversed direction"
		if got := strings.Contains(puml, notice); got != tc.notice {
			t.Errorf("%q: notice written %v, want %v:\n%s", tc.direction, got, tc.notice, puml)
		}
		checkPlantUMLSyntax(t, puml)
	}
}

// A palette fills the nodes a DOT palette fills, with the same fill per node,
// on every golden model and every palette; the border takes the family colour.
// A control node and a container keep the black-and-white rules.
func TestPlantUMLPaletteParityWithDOT(t *testing.T) {
	for _, tc := range plantumlGoldenCases {
		if tc.kind == KindSequence {
			continue
		}
		rendering := render(t, tc.file, tc.view)
		for _, palette := range Palettes() {
			dot, err := rendering.DOTWith(Options{Palette: palette})
			if err != nil {
				t.Fatalf("%s %s DOT: %v", tc.name, palette, err)
			}
			puml, err := rendering.PlantUMLWith(Options{Palette: palette})
			if err != nil {
				t.Fatalf("%s %s PlantUML: %v", tc.name, palette, err)
			}
			checkPlantUMLSyntax(t, puml)
			dotFills := map[string]string{}
			for _, line := range strings.Split(dot, "\n") {
				if m := dotFillLine.FindStringSubmatch(line); m != nil {
					dotFills[m[1]] = m[2]
				}
			}
			pumlFills := map[string]string{}
			for _, line := range strings.Split(puml, "\n") {
				if m := plantumlFillLine.FindStringSubmatch(line); m != nil {
					pumlFills[m[1]] = m[2]
				}
			}
			if len(dotFills) == 0 {
				t.Errorf("%s %s: DOT fills no node", tc.name, palette)
			}
			if fmt.Sprint(dotFills) != fmt.Sprint(pumlFills) {
				t.Errorf("%s %s: DOT fills %v, PlantUML %v", tc.name, palette, dotFills, pumlFills)
			}
		}
	}
}

// The palette goldens beside the DOT ones: the interconnection in okabe-ito
// is its black-and-white golden with fills and border colours added.
func TestGoldenPlantUMLPalettes(t *testing.T) {
	rendering := render(t, "interconnection.sysml", "PlantViews::loopView")
	puml, err := rendering.WriteWith(FormPlantUML, Options{Palette: PaletteOkabeIto})
	if err != nil {
		t.Fatalf("write plantuml: %v", err)
	}
	checkGolden(t, filepath.Join("testdata", "interconnection.okabe-ito.plantuml.golden"), puml)
	checkPlantUMLSyntax(t, puml)
	checkPlantUMLRenders(t, puml)
	plain, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	palette, bw := strings.Split(puml, "\n"), strings.Split(plain, "\n")
	if len(palette) != len(bw) {
		t.Fatalf("palette PlantUML has %d lines, black and white %d", len(palette), len(bw))
	}
	for i := range bw {
		if got := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(palette[i], bw[i])), " {"); palette[i] != bw[i] && !strings.HasPrefix(got, "#") {
			t.Errorf("line %d differs in more than fill:\n%s\n%s", i+1, palette[i], bw[i])
		}
	}
	for _, want := range []string{"as n1 <<part>> <<usage>> #F5D999;line:E69F00\n", "as n0 <<part def>> {\n"} {
		if !strings.Contains(puml, want) {
			t.Errorf("okabe-ito PlantUML lacks %q:\n%s", want, puml)
		}
	}
	if strings.Contains(plain, ";line:") {
		t.Errorf("black-and-white PlantUML fills a node:\n%s", plain)
	}
}

// A sequence palette colours the participants by keyword family, as a usage
// is filled in every other kind — the fill alone, a participant's border
// taking no colour of its own — and is not noted as unrepresented.
func TestPlantUMLSequencePaletteFillsParticipants(t *testing.T) {
	puml, err := render(t, "sequence.sysml", "SequenceViews::pubSubView").PlantUMLWith(Options{Palette: PaletteOkabeIto})
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	if got := strings.Count(puml, "<<part>> <<usage>> #F5D999\n"); got != 3 {
		t.Errorf("okabe-ito fills %d participants, want 3:\n%s", got, puml)
	}
	if strings.Contains(puml, "not represented: palette") {
		t.Errorf("sequence PlantUML reports the palette as unrepresented:\n%s", puml)
	}
	checkPlantUMLSyntax(t, puml)
	checkPlantUMLRenders(t, puml)
}

// A label carries its text as it is: a quote, a backslash, angle brackets and
// the creole escape are escaped, a run creole would read as markup is broken,
// and a newline is the `\n` PlantUML breaks a label at. Single marks stay legible.
func TestPlantUMLEscapesLabels(t *testing.T) {
	cases := map[string]string{
		`say "hi"`:               `say <U+0022>hi<U+0022>`,
		`a\b`:                    `a<U+005C>b`,
		"x<b>y":                  "x<U+003C>b<U+003E>y",
		"a**b //c d__e f--g":     "a<U+002A><U+002A>b <U+002F><U+002F>c d<U+005F><U+005F>e f<U+002D><U+002D>g",
		"[[link]] ~x":            "<U+005B><U+005B>link<U+005D><U+005D> <U+007E>x",
		"idle_to_moving [t > 0]": "idle_to_moving [t <U+003E> 0]",
		"line\nbreak":            `line\nbreak`,
		"a - b / c * d":          "a - b / c * d",
	}
	for in, want := range cases {
		if got := plantumlText(in); got != want {
			t.Errorf("plantumlText(%q) = %q, want %q", in, got, want)
		}
	}
	node := &Node{ID: "n0", Kind: "part", Name: `q"uote`, Type: "T<x>", Detail: "own **flow**"}
	label := `**q<U+0022>uote : T<U+003C>x<U+003E>**\n<size:10>//«part»//</size>\nown <U+002A><U+002A>flow<U+002A><U+002A>`
	if got := (&plantumlWriter{}).plantumlLabel(node); got != label {
		t.Errorf("plantumlLabel = %q, want %q", got, label)
	}
	rendering := &Rendering{Kind: KindTree, Roots: []*Node{node}}
	puml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	checkPlantUMLSyntax(t, puml)
	if !strings.Contains(puml, "class \""+label+"\" as n0 <<part>> <<usage>>\n") {
		t.Errorf("escaped label not quoted whole:\n%s", puml)
	}
}

// A node with no name is labelled by its kind alone, with no keyword line, as
// the other forms label it; a definition takes no usage stereotype.
func TestPlantUMLLabelShape(t *testing.T) {
	cases := []struct {
		node  *Node
		label string
		decor string
	}{
		{&Node{Kind: "part def", Name: "Vehicles::Vehicle"}, `**Vehicles::Vehicle**\n<size:10>//«part def»//</size>`, " <<part def>>"},
		{&Node{Kind: "part", Name: "engine", Type: "Engine"}, `**engine : Engine**\n<size:10>//«part»//</size>`, " <<part>> <<usage>>"},
		{&Node{Kind: "connect"}, `**connect**`, " <<connect>> <<usage>>"},
		{&Node{Kind: "state", Name: "off", Detail: "initial"}, `**off**\n<size:10>//«state»//</size>\ninitial`, " <<state>> <<usage>>"},
		{&Node{Kind: "fork", Name: "split"}, `**split**\n<size:10>//«fork»//</size>`, " <<fork>>"},
		{&Node{Kind: "initial", Name: "start"}, `**start**\n<size:10>//«initial»//</size>`, " <<start>>"},
		{&Node{Kind: "merge", Name: "m"}, `**m**\n<size:10>//«merge»//</size>`, " <<choice>>"},
		{&Node{Kind: "deep history", Name: "h"}, `**h**\n<size:10>//«deep history»//</size>`, " <<history*>>"},
		{&Node{Kind: "region", Name: "r"}, `**r**\n<size:10>//«region»//</size>`, " <<region>>"},
		{&Node{Kind: "library package", Name: "P"}, `**P**\n<size:10>//«library package»//</size>`, " <<library package>> <<package>>"},
		{&Node{Kind: "package", Name: "P"}, `**P**\n<size:10>//«package»//</size>`, " <<package>>"},
	}
	w := &plantumlWriter{}
	for _, tc := range cases {
		if got := w.plantumlLabel(tc.node); got != tc.label {
			t.Errorf("label of %+v = %q, want %q", tc.node, got, tc.label)
		}
		if got := w.decoration(tc.node); got != tc.decor {
			t.Errorf("decoration of %+v = %q, want %q", tc.node, got, tc.decor)
		}
	}
}

// The header is the Mermaid one with PlantUML's comment mark, the notices
// follow it, and the geometry is written as comments in the Mermaid shape.
func TestPlantUMLHeaderAndGeometryComments(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	puml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	mermaid := rendering.Mermaid()
	for _, line := range strings.Split(mermaid, "\n") {
		if comment, ok := strings.CutPrefix(line, "%% "); ok {
			if !strings.Contains(puml, "\n' "+comment+"\n") && !strings.HasPrefix(puml, "@startuml\n' "+comment+"\n") {
				t.Errorf("PlantUML lacks the comment %q:\n%s", comment, puml)
			}
		}
	}
	if !strings.Contains(puml, "\n' not represented: 2 positioned node(s) and 1 route(s) kept as comments; PlantUML pins no position, the dot form does\n") {
		t.Errorf("geometry is not noted as unrepresented:\n%s", puml)
	}
	if plain, _ := render(t, "interconnection.sysml", "PlantViews::loopView").PlantUML(); strings.Contains(plain, "kept as comments") {
		t.Errorf("a rendering without geometry notes it:\n%s", plain)
	}
	if !strings.Contains(puml, "\nhide stereotype\n' canvas: unit=px w=1200 h=800\n' layout: n1 x=300 y=40 collapsed\n' layout: n2 x=500 y=40 w=120 h=60\n' route: n1->n2 400,70 450,120 500,70\nrectangle ") {
		t.Errorf("geometry comments are not between the style and the body:\n%s", puml)
	}
	anonymous, err := (&Rendering{Kind: KindTree, Notices: []string{"one", "two"}}).PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	if !strings.HasPrefix(anonymous, "@startuml\n' tree rendering\n' not represented: one\n' not represented: two\n<style>\n") {
		t.Errorf("anonymous PlantUML:\n%s", anonymous)
	}
	if !strings.Contains(anonymous, "\nclass \"the rendering is empty: nothing the view exposes is shown by a tree rendering\" as empty\n@enduml\n") {
		t.Errorf("empty tree PlantUML:\n%s", anonymous)
	}
	checkPlantUMLSyntax(t, anonymous)
}

var (
	// plantumlArrowLine matches an arrow statement between two aliases.
	plantumlArrowLine = regexp.MustCompile(`^\s*(\[\*\]|\w+) (-\[[a-z=0-9]+\]->?|-->|->|--) (\w+)( : .*)?$`)
	// plantumlDeclarationLine matches an element declaration with its alias.
	plantumlDeclarationLine = regexp.MustCompile(`^\s*(class|rectangle|state|participant) ".*" as (\w+)( <<[^>]+>>)*( #[0-9A-F]{6}(;line:[0-9A-F]{6})?)?( \{)?$`)
	// dotFillLine and plantumlFillLine pick the fill a node is given in each form.
	dotFillLine      = regexp.MustCompile(`^\s*"([^"]+)" \[.*fillcolor="(#[0-9A-F]{6})"`)
	plantumlFillLine = regexp.MustCompile(`" as (\w+)(?: <<[^>]+>>)* (#[0-9A-F]{6})`)
)

// checkPlantUMLSyntax checks the shape of a PlantUML file: `@startuml` and
// `@enduml` bracket it, `<style>` is closed before the body, braces balance,
// every quoted label closes on its line, every declaration is well-formed,
// and every alias an arrow names is declared. It is a checker, not a parser.
func checkPlantUMLSyntax(t *testing.T, puml string) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(puml, "\n"), "\n")
	if lines[0] != "@startuml" || lines[len(lines)-1] != "@enduml" {
		t.Fatalf("PlantUML is not bracketed by @startuml/@enduml:\n%s", puml)
	}
	if !strings.HasSuffix(puml, "\n@enduml\n") {
		t.Fatalf("PlantUML does not end in a newline after @enduml")
	}
	declared := map[string]bool{"[*]": true}
	type arrow struct{ from, to string }
	var arrows []arrow
	depth, style := 0, false
	for i, line := range lines[1 : len(lines)-1] {
		switch {
		case line == "<style>":
			if style {
				t.Fatalf("line %d opens a second <style>:\n%s", i+2, puml)
			}
			style = true
			continue
		case line == "</style>":
			if !style {
				t.Fatalf("line %d closes no <style>:\n%s", i+2, puml)
			}
			style = false
			continue
		case style:
			if strings.Contains(line, "{") {
				depth++
			}
			if strings.Contains(line, "}") {
				depth--
			}
			continue
		case strings.HasPrefix(line, "'"):
			continue
		}
		if strings.Count(line, `"`)%2 != 0 {
			t.Fatalf("line %d leaves a quote open: %q", i+2, line)
		}
		if m := plantumlDeclarationLine.FindStringSubmatch(line); m != nil {
			declared[m[2]] = true
			if m[6] == " {" {
				depth++
			}
			continue
		}
		if m := plantumlArrowLine.FindStringSubmatch(line); m != nil {
			arrows = append(arrows, arrow{m[1], m[3]})
			continue
		}
		switch trimmed := strings.TrimSpace(line); {
		case trimmed == "}":
			depth--
			if depth < 0 {
				t.Fatalf("line %d closes more braces than opened:\n%s", i+2, puml)
			}
		case strings.HasPrefix(trimmed, "hide "), strings.HasPrefix(trimmed, "skinparam "), strings.HasSuffix(trimmed, " direction"):
		default:
			t.Fatalf("line %d is no statement the writer emits: %q\n%s", i+2, line, puml)
		}
	}
	if style {
		t.Fatalf("<style> is never closed:\n%s", puml)
	}
	if depth != 0 {
		t.Fatalf("braces do not balance (%d open):\n%s", depth, puml)
	}
	for _, a := range arrows {
		for _, end := range []string{a.from, a.to} {
			if !declared[end] {
				t.Errorf("arrow %s -> %s names the undeclared alias %s:\n%s", a.from, a.to, end, puml)
			}
		}
	}
}

// checkPlantUMLRenders asks a PlantUML jar to check the syntax when one is at
// hand, at the path OPENSYSML_PLANTUML_JAR names with `java` on the PATH, and
// is silent otherwise: nothing here depends on the jar.
func checkPlantUMLRenders(t *testing.T, puml string) {
	t.Helper()
	jar := os.Getenv("OPENSYSML_PLANTUML_JAR")
	if jar == "" {
		return
	}
	java, err := exec.LookPath("java")
	if err != nil {
		return
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "diagram.puml")
	if err := os.WriteFile(path, []byte(puml), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(java, "-Djava.awt.headless=true", "-jar", jar, "-checkonly", "-failfast2", path).CombinedOutput()
	if err != nil || strings.Contains(string(out), "Error") {
		t.Errorf("plantuml -checkonly: %v\n%s\n%s", err, out, puml)
	}
}
