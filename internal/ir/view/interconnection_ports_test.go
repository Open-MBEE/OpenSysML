package view

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// nodesByName indexes every node of a rendering by name.
func nodesByName(rendering *Rendering) map[string]*Node {
	out := map[string]*Node{}
	for _, node := range everyNode(rendering.Roots) {
		out[node.Name] = node
	}
	return out
}

// pinNamed is the port of node named name, nil when none is.
func pinNamed(node *Node, name string) *Port {
	if node == nil {
		return nil
	}
	for i := range node.Ports {
		if node.Ports[i].Name == name {
			return &node.Ports[i]
		}
	}
	return nil
}

// checkToasterPorts asserts a rendering rooted at Toaster pins on each typed
// part the port its definition declares, named and typed as declared, and joins
// the interface and the flow at the pins rather than at the parts.
func checkToasterPorts(t *testing.T, rendering *Rendering) {
	t.Helper()
	if len(rendering.Roots) != 1 {
		t.Fatalf("roots = %d, want 1: %v", len(rendering.Roots), nodeNames(rendering.Roots))
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %q, want none", rendering.Notices)
	}
	byName := nodesByName(rendering)
	heating, control, chassis := byName["heating"], byName["control"], byName["chassis"]
	in, out := pinNamed(heating, "durationIn"), pinNamed(control, "durationOut")
	if in == nil || out == nil {
		t.Fatalf("the parts pin no port: heating %+v, control %+v", heating, control)
	}
	if in.Type != "~DurationPort" || in.Direction != PortUndirected || in.ID != heating.ID+".0" {
		t.Errorf("durationIn = %+v, want typed ~DurationPort, undirected, pin 0 of heating", *in)
	}
	if out.Type != "DurationPort" || out.Direction != PortUndirected {
		t.Errorf("durationOut = %+v, want typed DurationPort, undirected", *out)
	}
	if in.Origin.Doc == "" || in.Origin.Name.Len == 0 {
		t.Errorf("durationIn has no origin: %+v", in.Origin)
	}
	if chassis == nil || len(chassis.Ports) != 0 || len(chassis.Children) != 0 {
		t.Errorf("chassis, whose definition has no port, = %+v, want a plain node", chassis)
	}
	for _, node := range everyNode(rendering.Roots) {
		if len(node.Children) != 0 && node != rendering.Roots[0] {
			t.Errorf("node %s nests %v, want the ports pinned, not nested", node.Name, nodeNames(node.Children))
		}
	}
	if len(rendering.Edges) != 2 {
		t.Fatalf("edges = %+v, want the interface and the flow", rendering.Edges)
	}
	for _, edge := range rendering.Edges {
		if edge.From != control.ID || edge.To != heating.ID || edge.FromPort != out.ID || edge.ToPort != in.ID {
			t.Errorf("edge %q = %s:%s -> %s:%s, want %s:%s -> %s:%s", edge.Label,
				edge.From, edge.FromPort, edge.To, edge.ToPort, control.ID, out.ID, heating.ID, in.ID)
		}
	}
	if rendering.Edges[0].Kind != EdgeConnection || rendering.Edges[0].Label != "durationInterface" {
		t.Errorf("edge 0 = %+v, want the interface", rendering.Edges[0])
	}
	if rendering.Edges[1].Kind != EdgeFlow || rendering.Edges[1].Label != "of Real" {
		t.Errorf("edge 1 = %+v, want the flow", rendering.Edges[1])
	}
}

// A part typed by a definition pins the ports the definition declares on its
// border, named and typed as declared, in every form; a connector between such
// ports ends at the pins, keeping its label. A part whose definition has no
// port draws as before.
func TestTypedPartsPinTheirDefinitionsPorts(t *testing.T) {
	rendering := render(t, "interconnection-ports.sysml", "ToasterViews::toasterView")
	checkToasterPorts(t, rendering)
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	cameo, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("cameo DOT: %v", err)
	}
	plantuml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	// PlantUML writes the tilde, a creole marker, as its code point.
	forms := map[string][2]string{
		"DOT":       {dot, "durationIn : ~DurationPort"},
		"cameo DOT": {cameo, "durationIn : ~DurationPort"},
		"PlantUML":  {plantuml, "durationIn : <U+007E>DurationPort"},
		"text":      {rendering.Text(), "durationIn : ~DurationPort"},
	}
	for label, form := range forms {
		for _, want := range []string{form[1], "durationOut : DurationPort", "durationInterface", "of Real"} {
			if !strings.Contains(form[0], want) {
				t.Errorf("%s lacks %q:\n%s", label, want, form[0])
			}
		}
		for _, want := range []string{"durationIn :", "durationOut :"} {
			if got := strings.Count(form[0], want); got != 1 {
				t.Errorf("%s names %q %d times, want once:\n%s", label, want, got, form[0])
			}
		}
	}
	for _, want := range []string{`"n2":"n2.0" -> "n1":"n1.0" [label="durationInterface"`, `"n2":"n2.0" -> "n1":"n1.0" [label="of Real"`} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks the edge at the ports %q:\n%s", want, dot)
		}
	}
	for _, want := range []string{"n2.0 -[thickness=3]- n1.0 : durationInterface", "n2.0 -[dashed]-> n1.0 : of Real",
		"port \"durationIn : <U+007E>DurationPort\" as n1.0"} {
		if !strings.Contains(plantuml, want) {
			t.Errorf("PlantUML lacks %q:\n%s", want, plantuml)
		}
	}
	checkPlantUMLRenders(t, plantuml)
	for _, want := range []string{"  part heating : HeatingSystem\n    port durationIn : ~DurationPort\n",
		"  control.durationOut -- heating.durationIn: durationInterface\n", "  control.durationOut => heating.durationIn: of Real\n"} {
		if !strings.Contains(forms["text"][0], want) {
			t.Errorf("text lacks %q:\n%s", want, forms["text"][0])
		}
	}
}

// A pseudo-view rooted at a part def draws the same: the parts with the ports
// their definitions declare pinned on them, joined at the pins. Rooted at a part
// def declaring ports itself, the ports are its own nested nodes, as before.
func TestPseudoInterconnectionOfAPartDefPinsThePorts(t *testing.T) {
	r, idx := loadFixture(t, "interconnection-ports.sysml")
	rendering, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, "ToasterDemo::Toaster")}, KindInterconnection, "")
	if err != nil {
		t.Fatalf("RenderExposed: %v", err)
	}
	checkToasterPorts(t, rendering)

	rendering, err = r.RenderExposed([]*symbols.Symbol{lookup(t, idx, "ToasterDemo::HeatingSystem")}, KindInterconnection, "")
	if err != nil {
		t.Fatalf("RenderExposed: %v", err)
	}
	root := rendering.Roots[0]
	if len(rendering.Roots) != 1 || len(root.Children) != 1 || root.Children[0].Name != "durationIn" || len(root.Ports) != 0 {
		t.Errorf("HeatingSystem draws %v with ports %v, want its port durationIn nested and nothing pinned",
			nodeNames(rendering.Roots), root.Ports)
	}
}

// Two usages of one definition each pin the definition's port as their own, and
// a connector to one usage's port ends at that usage's pin.
func TestEachUsagePinsItsDefinitionsPorts(t *testing.T) {
	rendering := render(t, "interconnection-ports.sysml", "ToasterViews::dualView")
	byName := nodesByName(rendering)
	primary, backup, heating := byName["primary"], byName["backup"], byName["heating"]
	out, standby, in := pinNamed(primary, "durationOut"), pinNamed(backup, "durationOut"), pinNamed(heating, "durationIn")
	if out == nil || standby == nil || in == nil {
		t.Fatalf("the usages pin no port: %+v %+v %+v", primary, backup, heating)
	}
	if out.ID == standby.ID {
		t.Errorf("primary and backup share the pin %s", out.ID)
	}
	if len(rendering.Edges) != 1 || rendering.Edges[0].From != backup.ID || rendering.Edges[0].FromPort != standby.ID ||
		rendering.Edges[0].To != heating.ID || rendering.Edges[0].ToPort != in.ID {
		t.Errorf("edges = %+v, want standby from backup's pin %s to heating's %s", rendering.Edges, standby.ID, in.ID)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %q, want none", rendering.Notices)
	}
}

// A definition exposed beside a usage of it draws its own port as a nested
// node, and the usage pins the port as its own: a connector naming the usage's
// port ends at the usage's pin, not at the definition's node.
func TestAUsageEndIsTheUsagesPortBesideItsDefinition(t *testing.T) {
	rendering := render(t, "interconnection-ports.sysml", "ToasterViews::besideView")
	if len(rendering.Roots) != 2 {
		t.Fatalf("roots = %d, want 2: %v", len(rendering.Roots), nodeNames(rendering.Roots))
	}
	def := rendering.Roots[0]
	heating := nodesByName(rendering)["heating"]
	if len(def.Children) != 1 || def.Children[0].Name != "durationIn" {
		t.Fatalf("the definition draws %v, want its port nested", nodeNames(def.Children))
	}
	in := pinNamed(heating, "durationIn")
	if in == nil {
		t.Fatalf("the usage pins no port: %+v", heating)
	}
	if len(rendering.Edges) != 2 {
		t.Fatalf("edges = %+v, want the interface and the flow", rendering.Edges)
	}
	for _, edge := range rendering.Edges {
		if edge.To != heating.ID || edge.ToPort != in.ID {
			t.Errorf("edge %q ends at %s:%s, want the usage's pin %s:%s", edge.Label, edge.To, edge.ToPort, heating.ID, in.ID)
		}
	}
}

// A composite part — one nesting parts of its own — pins the ports its
// definition declares as a plain one does, and Mermaid, which draws it as a
// subgraph, names them in the subgraph's title. A connector the part owns that
// names such a port bare ends at the part's pin, not at the port its definition
// draws nested when the view exposes the definition too.
func TestACompositePartPinsItsPortsAndEndsItsBareEndsAtThem(t *testing.T) {
	rendering := render(t, "interconnection-ports.sysml", "ToasterViews::rigView")
	if len(rendering.Roots) != 2 {
		t.Fatalf("roots = %d, want 2: %v", len(rendering.Roots), nodeNames(rendering.Roots))
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %q, want none", rendering.Notices)
	}
	def := rendering.Roots[0]
	if len(def.Children) != 1 || def.Children[0].Name != "reading" {
		t.Fatalf("the definition draws %v, want its port nested", nodeNames(def.Children))
	}
	byName := nodesByName(rendering)
	sensor, heating := byName["sensor"], byName["heating"]
	reading, in := pinNamed(sensor, "reading"), pinNamed(heating, "durationIn")
	if reading == nil || in == nil {
		t.Fatalf("the parts pin no port: sensor %+v, heating %+v", sensor, heating)
	}
	if len(sensor.Children) != 1 || sensor.Children[0].Name != "probe" {
		t.Errorf("sensor nests %v, want probe", nodeNames(sensor.Children))
	}
	if len(rendering.Edges) != 1 {
		t.Fatalf("edges = %+v, want feed", rendering.Edges)
	}
	if edge := rendering.Edges[0]; edge.From != sensor.ID || edge.FromPort != reading.ID || edge.To != heating.ID || edge.ToPort != in.ID {
		t.Errorf("feed = %s:%s -> %s:%s, want %s:%s -> %s:%s", edge.From, edge.FromPort, edge.To, edge.ToPort,
			sensor.ID, reading.ID, heating.ID, in.ID)
	}
	mermaid := rendering.Mermaid()
	if want := "subgraph " + sensor.ID + " [\"sensor : Sensor<br>«part»<br>port reading : DurationPort\"]"; !strings.Contains(mermaid, want) {
		t.Errorf("Mermaid lacks %q:\n%s", want, mermaid)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if want := fmt.Sprintf("%q -> %q:%q [label=\"feed\"", reading.ID, heating.ID, in.ID); !strings.Contains(dot, want) {
		t.Errorf("DOT lacks the edge at the pins %q:\n%s", want, dot)
	}
}
