package view

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoldenTreeCompartmentsAutomatic locks every form of an automatically
// laid-out tree under the `most` and `ports` foldings: the owned features of
// the controller model as compartment rows of their definitions, and the
// hub's ports with the ports nested under them.
func TestGoldenTreeCompartmentsAutomatic(t *testing.T) {
	for _, tc := range []struct {
		mode Compartments
		view string
	}{{CompartmentsMost, "controllers"}, {CompartmentsPorts, "hub"}} {
		rendering := render(t, "tree-compartments-most.sysml", "CompartmentDemoViews::"+tc.view)
		options := Options{Compartments: tc.mode}
		for _, form := range []Form{FormDot, FormMermaid, FormPlantUML, FormD2, FormText} {
			out, err := rendering.WriteWith(form, options)
			if err != nil {
				t.Fatalf("write %s %s: %v", tc.mode, form, err)
			}
			checkGolden(t, filepath.Join("testdata", "tree-compartments-"+string(tc.mode)+"."+string(form)+".golden"), out)
		}
		cameo, err := rendering.DOTWith(Options{Compartments: tc.mode, Style: StyleCameo})
		if err != nil {
			t.Fatalf("DOTWith(cameo, %s): %v", tc.mode, err)
		}
		checkGolden(t, filepath.Join("testdata", "tree-compartments-"+string(tc.mode)+".cameo.dot.golden"), cameo)
		plain, err := rendering.WriteWith(FormDot, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if folded, _ := rendering.WriteWith(FormDot, Options{Compartments: CompartmentsPositioned}); folded != plain {
			t.Errorf("%s: the positioned folding changed an unpositioned tree", tc.view)
		}
	}
}

// TestTreeCompartmentsMostFold checks the `most` predicate on the issue's
// model: every owned usage folds, the definitions stay nodes, the folded
// part's own members are left undrawn, and the composition edge the folded
// part stands for stays at its owner with no containment edge beside it.
func TestTreeCompartmentsMostFold(t *testing.T) {
	rendering := render(t, "tree-compartments-most.sysml", "CompartmentDemoViews::controllers")
	folded := foldCompartments(rendering, CompartmentsMost)
	if folded == rendering {
		t.Fatal("foldCompartments(most) returned the rendering unchanged")
	}
	if got := compartmentText(folded.Roots[0]); got != "attributes: label : String, reading : Real" {
		t.Errorf("Sensor compartments = %q", got)
	}
	if got := compartmentText(folded.Roots[1]); got != "attributes: enabled : Boolean; parts: sensor : Sensor" {
		t.Errorf("Controller compartments = %q", got)
	}
	for _, root := range folded.Roots {
		if len(root.Children) != 0 {
			t.Errorf("%s keeps %d children as nodes", root.Name, len(root.Children))
		}
	}
	if len(folded.Edges) != 1 || folded.Edges[0].Kind != EdgeComposition || folded.Edges[0].From != folded.Roots[1].ID || folded.Edges[0].To != folded.Roots[0].ID || folded.Edges[0].ToLabel != "sensor" {
		t.Errorf("edges = %+v, want one composition Controller -> Sensor labelled sensor", folded.Edges)
	}
	want := "4 member(s) drawn as compartment rows of their owner, and 2 node(s) under them left undrawn"
	if len(folded.Notices) != 1 || folded.Notices[0] != want {
		t.Errorf("notices = %q, want %q", folded.Notices, want)
	}
	if len(rendering.Roots[1].Children) != 2 || len(rendering.Roots[1].Compartments) != 0 {
		t.Error("folding changed the rendering it was given")
	}
}

// TestTreeCompartmentsFoldKeepsNodes checks what every automatic folding
// leaves a node: a usage owning a nested definition, a positioned member and
// a stand-in under `most`, and every non-port member under `ports`, whose
// nested ports list under their port by path.
func TestTreeCompartmentsFoldKeepsNodes(t *testing.T) {
	hub := render(t, "tree-compartments-most.sysml", "CompartmentDemoViews::hub")
	most := foldCompartments(hub, CompartmentsMost)
	if got := keptNames(most.Roots[0]); got != "slot" {
		t.Errorf("most keeps %q as nodes, want the part owning a definition", got)
	}
	if got := compartmentText(most.Roots[0]); got != "attributes: channels : Natural; parts: buffer : Buffer; ports: input : DataPort, input.clock : ClockPort, output : DataPort, output.sync : ClockPort, output.clock : ClockPort" {
		t.Errorf("most Hub compartments = %q", got)
	}
	ports := foldCompartments(hub, CompartmentsPorts)
	if got := keptNames(ports.Roots[0]); got != "channels,buffer,slot" {
		t.Errorf("ports keeps %q as nodes, want every non-port member", got)
	}
	if got := compartmentText(ports.Roots[0]); got != "ports: input : DataPort, input.clock : ClockPort, output : DataPort, output.sync : ClockPort, output.clock : ClockPort" {
		t.Errorf("ports Hub compartments = %q", got)
	}
	want := "5 member(s) drawn as compartment rows of their owner, and 5 node(s) under them left undrawn"
	if len(ports.Notices) != 1 || ports.Notices[0] != want {
		t.Errorf("ports notices = %q, want %q", ports.Notices, want)
	}
	positioned := render(t, "tree-compartments.sysml", "AmplifierViews::positioned")
	folded := foldCompartments(positioned, CompartmentsMost)
	if got := keptNames(folded.Roots[0]); got != "housing,shield,Slot" {
		t.Errorf("most keeps %q of the positioned Stage, want its placed parts and its nested definition", got)
	}
	if got := compartmentText(folded.Roots[0]); !strings.HasPrefix(got, "attributes: gain : dB = 15.0, noiseFigure : Real = RealFunctions::sqrt(4.0); parts: heatsink : Heatsink; ports: input : SignalPort, output : SignalPort") {
		t.Errorf("most Stage compartments = %q", got)
	}
	owner := &Node{ID: "n0", Kind: "part def", Name: "Hub", Children: []*Node{
		{ID: "n1", Kind: "port", Name: "made", StandIn: true},
		{ID: "n2", Kind: "port", Name: "real"},
	}}
	standIns := &Rendering{Kind: KindTree, Shown: DefaultShown, Roots: []*Node{owner}}
	for _, mode := range []Compartments{CompartmentsMost, CompartmentsPorts} {
		kept := foldCompartments(standIns, mode)
		if got := keptNames(kept.Roots[0]); got != "made" || compartmentText(kept.Roots[0]) != "ports: real" {
			t.Errorf("%s keeps %q as nodes with rows %q, want the stand-in kept and the port folded", mode, got, compartmentText(kept.Roots[0]))
		}
	}
	exposed := &Node{ID: "n0", Kind: "view", Name: "wheels", Children: []*Node{
		{ID: "n1", Kind: "part", Name: "vehicle", Children: []*Node{{ID: "n2", Kind: "part", Name: "wheel"}}},
	}}
	nested := &Rendering{Kind: KindTree, Shown: DefaultShown, Roots: []*Node{exposed}}
	for _, mode := range []Compartments{CompartmentsMost, CompartmentsPorts} {
		kept := foldCompartments(nested, mode)
		if got := keptNames(kept.Roots[0]); got != "vehicle" || len(kept.Roots[0].Compartments) != 0 {
			t.Errorf("%s keeps %q under the view with rows %q, want what the view exposes kept as nodes", mode, got, compartmentText(kept.Roots[0]))
		}
	}
}

// TestTreeCompartmentsReanchor checks that an edge at a folded member moves
// to the member's owner, is drawn once however many rows it reaches from, and
// is dropped when both its ends fall inside one owner.
func TestTreeCompartmentsReanchor(t *testing.T) {
	owner := &Node{ID: "n0", Kind: "part def", Name: "Hub"}
	a := &Node{ID: "n1", Kind: "port", Name: "a"}
	b := &Node{ID: "n2", Kind: "port", Name: "b"}
	other := &Node{ID: "n3", Kind: "part def", Name: "Other"}
	owner.Children = []*Node{a, b}
	r := &Rendering{Kind: KindTree, Shown: DefaultShown, Roots: []*Node{owner, other}, Edges: []Edge{
		{From: "n1", To: "n3", Kind: EdgeReference},
		{From: "n2", To: "n3", Kind: EdgeReference},
		{From: "n1", To: "n2", Kind: EdgeConnection},
		{From: "n0", To: "n3", Kind: EdgeComposition},
	}, Notes: []Note{{Text: "a note", Anchor: "n1"}, {Text: "an edge note", EdgeFrom: "n1", EdgeTo: "n2"}}}
	folded := foldCompartments(r, CompartmentsPorts)
	if len(folded.Edges) != 2 || folded.Edges[0].From != "n0" || folded.Edges[0].To != "n3" || folded.Edges[1].From != "n0" || folded.Edges[1].Kind != EdgeComposition {
		t.Errorf("edges = %+v, want the reference moved to Hub once and the composition kept", folded.Edges)
	}
	if len(folded.Notes) != 2 || folded.Notes[0].Anchor != "n0" || folded.Notes[1] != (Note{Text: "an edge note", Anchor: "n0"}) {
		t.Errorf("notes = %+v, want both notes moved to Hub, the one on the dropped edge as a note on Hub", folded.Notes)
	}
	want := "2 member(s) drawn as compartment rows of their owner, and 1 edge(s) at them moved to their owner, and 2 edge(s) within one owner dropped, and 2 note(s) at them moved to their owner"
	if len(folded.Notices) != 1 || folded.Notices[0] != want {
		t.Errorf("notices = %q, want %q", folded.Notices, want)
	}
	if r.Edges[0].From != "n1" || len(owner.Children) != 2 {
		t.Error("folding changed the rendering it was given")
	}
}

// TestCompartmentsOption checks the option's parsing and the error an unknown
// folding raises from every form.
func TestCompartmentsOption(t *testing.T) {
	if CompartmentsNames() != "positioned, most, ports" {
		t.Errorf("CompartmentsNames() = %q", CompartmentsNames())
	}
	if _, ok := ParseCompartments("rows"); ok {
		t.Error("ParseCompartments accepted an unknown folding")
	}
	rendering := render(t, "tree-compartments-most.sysml", "CompartmentDemoViews::controllers")
	for _, form := range []Form{FormDot, FormMermaid, FormPlantUML, FormD2, FormText} {
		_, err := rendering.WriteWith(form, Options{Compartments: "rows"})
		var unknown *UnknownCompartmentsError
		if !errors.As(err, &unknown) || unknown.Name != "rows" || !errors.Is(err, ErrUnknownCompartments) {
			t.Errorf("%s with an unknown folding: %v", form, err)
		}
	}
	data := rendering.DataWith(Options{Compartments: CompartmentsMost})
	var rows []string
	for _, node := range data.Nodes {
		if node.Compartment != "" {
			rows = append(rows, node.Parent+"/"+node.Compartment+"/"+node.Name)
		}
	}
	if got := strings.Join(rows, " "); got != "n0/attributes/label n0/attributes/reading n3/attributes/enabled n3/parts/sensor" {
		t.Errorf("data rows = %q", got)
	}
}

func compartmentText(node *Node) string {
	l := labeller{shown: DefaultShown}
	var parts []string
	for _, compartment := range node.Compartments {
		var rows []string
		for _, row := range compartment.Rows {
			rows = append(rows, l.rowText(row))
		}
		parts = append(parts, compartment.Name+": "+strings.Join(rows, ", "))
	}
	return strings.Join(parts, "; ")
}

func keptNames(node *Node) string {
	var names []string
	for _, child := range node.Children {
		names = append(names, child.Name)
	}
	return strings.Join(names, ",")
}
