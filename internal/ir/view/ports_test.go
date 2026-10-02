package view

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The port displays are named, the minimal one by the empty name too, and a
// name none has is refused on every form by an error naming the displays.
func TestPortsAreNamed(t *testing.T) {
	for name, want := range map[string]Ports{"": PortsMinimal, "minimal": PortsMinimal, "full": PortsFull} {
		if got, ok := ParsePorts(name); !ok || got != want {
			t.Errorf("ParsePorts(%q) = %q, %v, want %q", name, got, ok, want)
		}
	}
	if _, ok := ParsePorts("all"); ok {
		t.Errorf("ParsePorts accepts \"all\"")
	}
	if got := PortsNames(); got != "minimal, full" {
		t.Errorf("PortsNames() = %q", got)
	}
	rendering := render(t, "interconnection-ports.sysml", "ToasterViews::toasterView")
	for _, form := range Forms() {
		_, err := rendering.WriteWith(form, Options{Ports: Ports("all")})
		var unknown *UnknownPortsError
		if !errors.As(err, &unknown) || !errors.Is(err, ErrUnknownPorts) || unknown.Name != "all" {
			t.Errorf("%s with ports \"all\": err = %v, want an UnknownPortsError", form, err)
		} else if !strings.Contains(err.Error(), "minimal, full") {
			t.Errorf("%s: %v does not name the displays", form, err)
		}
	}
	for _, write := range map[string]func(Options) (string, error){"DOT": rendering.DOTWith, "PlantUML": rendering.PlantUMLWith} {
		if _, err := write(Options{Ports: Ports("all")}); !errors.Is(err, ErrUnknownPorts) {
			t.Errorf("err = %v, want ErrUnknownPorts", err)
		}
	}
}

// By default a part draws the ports a connector of the view ends at and no
// other: of two usages of one definition, the one connected keeps its pin and
// the other draws plain, in every form, and the connector ends at the pin.
func TestTheMinimalDisplayDrawsTheConnectedPortsAlone(t *testing.T) {
	rendering := render(t, "interconnection-ports.sysml", "ToasterViews::dualView")
	byName := nodesByName(rendering)
	primary, backup, heating := byName["primary"], byName["backup"], byName["heating"]
	idle, standby, in := pinNamed(primary, "durationOut"), pinNamed(backup, "durationOut"), pinNamed(heating, "durationIn")
	if idle == nil || standby == nil || in == nil {
		t.Fatalf("the usages pin no port: %+v %+v %+v", primary, backup, heating)
	}
	ports := rendering.portView(PortsMinimal)
	if got := ports.of(primary); len(got) != 0 {
		t.Errorf("primary draws %v, want no pin: none is connected", got)
	}
	if got := ports.of(backup); len(got) != 1 || got[0].ID != standby.ID {
		t.Errorf("backup draws %v, want its connected pin %s", got, standby.ID)
	}
	if got := rendering.portView(PortsFull).of(primary); len(got) != 1 || got[0].ID != idle.ID {
		t.Errorf("the full display draws %v of primary, want its pin %s", got, idle.ID)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	plantuml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	mermaid := rendering.Mermaid()
	text := rendering.Text()
	for label, form := range map[string]string{"DOT": dot, "PlantUML": plantuml, "Mermaid": mermaid, "text": text} {
		if strings.Contains(form, idle.ID) {
			t.Errorf("%s draws the unconnected pin %s:\n%s", label, idle.ID, form)
		}
		// The text form names the pin again at the connection's end.
		if got := strings.Count(form, "durationOut"); got != map[bool]int{false: 1, true: 2}[label == "text"] {
			t.Errorf("%s names durationOut %d times, want once, at backup:\n%s", label, got, form)
		}
	}
	// The unconnected part is a plain node, as a part without ports is.
	for label, want := range map[string]string{
		"DOT":      `"` + primary.ID + `" [style="rounded,filled", label=<<font point-size="10"><i>«part»</i></font><br/><b>primary : ControlSystem</b>>];`,
		"PlantUML": "  rectangle \"<size:10>//«part»//</size>\\n**primary : ControlSystem**\" as " + primary.ID + " <<part>> <<usage>>\n",
		"Mermaid":  "    " + primary.ID + "[\"«part»<br>primary : ControlSystem\"]\n",
		"text":     "  part primary : ControlSystem\n  part backup : ControlSystem\n    port durationOut\n",
	} {
		if !strings.Contains(map[string]string{"DOT": dot, "PlantUML": plantuml, "Mermaid": mermaid, "text": text}[label], want) {
			t.Errorf("%s lacks %q", label, want)
		}
	}
	// The connected pins are squares: a cell of the part's label in DOT, a
	// `port` in PlantUML, a node in the part's subgraph in Mermaid, each ending
	// the connector.
	for label, wants := range map[string][]string{
		"DOT": {`"` + backup.ID + `" [shape=plain, label=<<table border="0" cellborder="0" cellspacing="0" cellpadding="0">`,
			`<td port="` + standby.ID + `" border="1" fixedsize="true" width="10" height="10" bgcolor="white"></td><td align="left"><font point-size="8">durationOut</font></td>`,
			`"` + backup.ID + `":"` + standby.ID + `" -> "` + heating.ID + `":"` + in.ID + `" [label="standby"`},
		"PlantUML": {"    port \"durationOut\" as " + standby.ID + "\n", standby.ID + " -[thickness=3]- " + in.ID + " : standby\n"},
		"Mermaid":  {"      " + standby.ID + "[\"durationOut\"]\n", "  " + standby.ID + " ---|\"standby\"| " + in.ID + "\n"},
	} {
		for _, want := range wants {
			if !strings.Contains(map[string]string{"DOT": dot, "PlantUML": plantuml, "Mermaid": mermaid}[label], want) {
				t.Errorf("%s lacks %q:\n%s", label, want, map[string]string{"DOT": dot, "PlantUML": plantuml, "Mermaid": mermaid}[label])
			}
		}
	}
	full, err := rendering.WriteWith(FormDot, Options{Ports: PortsFull})
	if err != nil {
		t.Fatalf("full DOT: %v", err)
	}
	if got := strings.Count(full, "durationOut : DurationPort"); got != 2 {
		t.Errorf("the full display names durationOut : DurationPort %d times, want at primary and backup:\n%s", got, full)
	}
}

// A view mixing a connector at ports with one between the parts themselves
// draws the pins the former ends at and leaves the latter at the parts, in
// every form, and a part whose only port no connector reaches draws plain.
func TestPortedAndPinlessEdgesMixUnderTheMinimalDisplay(t *testing.T) {
	rendering := render(t, "interconnection.sysml", "PlantViews::loopView")
	byName := nodesByName(rendering)
	pump, tank, sensor := byName["pump"], byName["tank"], byName["sensor"]
	outlet, inlet := pinNamed(pump, "outlet"), pinNamed(tank, "inlet")
	if outlet == nil || inlet == nil || pinNamed(sensor, "outlet") == nil {
		t.Fatalf("the parts pin no port: %+v %+v %+v", pump, tank, sensor)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	plantuml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	mermaid := rendering.Mermaid()
	for label, wants := range map[string][]string{
		"DOT": {`"` + pump.ID + `":"` + outlet.ID + `" -> "` + tank.ID + `":"` + inlet.ID + `" [label="supply"`,
			`"` + pump.ID + `" -> "` + tank.ID + `" [label="of Water"`,
			`"` + sensor.ID + `" [style="rounded,filled", label=<<font point-size="10"><i>«part»</i></font><br/><b>sensor : Pump</b>>];`},
		"PlantUML": {outlet.ID + " -[thickness=3]- " + inlet.ID + " : supply\n", pump.ID + " -[dashed]-> " + tank.ID + " : of Water\n",
			"  rectangle \"<size:10>//«part»//</size>\\n**sensor : Pump**\" as " + sensor.ID + " <<part>> <<usage>>\n"},
		"Mermaid": {"  " + outlet.ID + " ---|\"supply\"| " + inlet.ID + "\n", "  " + pump.ID + " -.->|\"of Water\"| " + tank.ID + "\n",
			"    " + sensor.ID + "[\"«part»<br>sensor : Pump\"]\n"},
	} {
		form := map[string]string{"DOT": dot, "PlantUML": plantuml, "Mermaid": mermaid}[label]
		for _, want := range wants {
			if !strings.Contains(form, want) {
				t.Errorf("%s lacks %q:\n%s", label, want, form)
			}
		}
		if got := strings.Count(form, "outlet"); got != 1 {
			t.Errorf("%s names outlet %d times, want once, at pump:\n%s", label, got, form)
		}
	}
}

// A kind without a port display draws every pin whatever the display asks: an
// action's pins are the flows' ends, named in full.
func TestAnActionDrawsEveryPinWhateverTheDisplay(t *testing.T) {
	rendering := &Rendering{View: "V", Kind: KindAction,
		Roots: []*Node{
			{ID: "A", Kind: "action", Name: "a", Ports: []Port{{ID: "A.0", Name: "result", Type: "Real", Direction: PortOut}, {ID: "A.1", Name: "spare", Direction: PortOut}}},
			{ID: "B", Kind: "action", Name: "b", Ports: []Port{{ID: "B.0", Name: "input", Direction: PortIn}}},
		},
		Edges: []Edge{{From: "A", To: "B", FromPort: "A.0", ToPort: "B.0", Kind: EdgeFlow}},
	}
	for _, display := range []Ports{PortsMinimal, PortsFull} {
		ports := rendering.portView(display)
		if ports.minimal || len(ports.of(rendering.Roots[0])) != 2 || ports.pinLabel(rendering.Roots[0].Ports[0]) != "result : Real" {
			t.Errorf("%s: an action's ports are %+v, want both, typed", display, ports.of(rendering.Roots[0]))
		}
	}
}

// The minimal display draws the pins of the edges the DOT form draws, not of
// those it leaves out: a positioned drawing omits an unplaced part and the
// connectors at it, so the placed part's port those alone reached is omitted
// too, not left a pin nothing ends at; the strip, drawing every node, draws it.
func TestTheMinimalDisplayFollowsTheEdgesDOTDraws(t *testing.T) {
	rendering := render(t, "interconnection-ports.sysml", "ToasterViews::placedView")
	omitted, err := rendering.DOTWith(Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"durationIn", "durationOut", "control", `port="n`} {
		if strings.Contains(omitted, unwanted) {
			t.Errorf("the omitted connectors' %s is drawn:\n%s", unwanted, omitted)
		}
	}
	for _, want := range []string{"heating", "chassis", "1 node(s) without a position, left undrawn, and 2 edge(s) at them"} {
		if !strings.Contains(omitted, want) {
			t.Errorf("the drawing lacks %q:\n%s", want, omitted)
		}
	}
	stripped, err := rendering.DOTWith(Options{Unplaced: UnplacedStrip})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`xlabel="durationIn"`, `xlabel="durationOut"`, `"n2.0" -> "n1.0" [label="durationInterface"`} {
		if !strings.Contains(stripped, want) {
			t.Errorf("the strip lacks %q:\n%s", want, stripped)
		}
	}
}

// A pinned node's table is as wide as its body: each pin row holds exactly the
// cells the body spans, so the box runs the width of the node.
func TestAPinnedDOTNodeSpansItsPinRows(t *testing.T) {
	rendering := render(t, "interconnection-ports.sysml", "ToasterViews::toasterView")
	source, err := rendering.DOTWith(Options{})
	if err != nil {
		t.Fatal(err)
	}
	pinned := 0
	for _, line := range strings.Split(source, "\n") {
		if !strings.Contains(line, "shape=plain") {
			continue
		}
		pinned++
		span := regexp.MustCompile(`colspan="(\d+)"`).FindStringSubmatch(line)
		if span == nil {
			t.Fatalf("no body colspan in %s", line)
		}
		columns, _ := strconv.Atoi(span[1])
		for _, row := range strings.Split(line, "<tr>")[1:] {
			if !strings.Contains(row, "port=") {
				continue
			}
			if cells := strings.Count(row, "<td"); cells != columns {
				t.Errorf("a pin row of %d cells under a body spanning %d:\n%s", cells, columns, line)
			}
		}
	}
	if pinned != 2 {
		t.Errorf("%d pinned nodes, want heating and control", pinned)
	}
}
