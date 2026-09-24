package view

import (
	"strings"
	"testing"
)

// edgeLabelModel declares one edge of each kind twice: named with no other text
// of its own, and named alongside the text it otherwise carries.
const edgeLabelModel = `package Labels {
	item def Water;
	part def Pump { port outlet; port level; }
	part def Tank { port inlet; port level; }
	part def Loop {
		part pump : Pump;
		part tank : Tank;
		connection supply connect pump.outlet to tank.inlet;
		binding 'pump.level = tank.level' bind pump.level = tank.level;
		flow 'pump.outlet to tank.inlet' of Water from pump.outlet to tank.inlet;
	}
	attribute def Sig;
	state def Machine {
		entry; then off;
		state off;
		state on;
		state idle;
		transition 'off then on' first off then on;
		transition 'on accept Sig then idle' first on accept Sig then idle;
	}
	action def Drive {
		action a { out o; }
		action b { in i; }
		action c;
		succession 'start to a' first start then a;
		succession 'a to b' first a if true then b;
		succession 'b to c' first b then c;
		flow 'a.o to b.i' from a.o to b.i;
		succession 'c to done' first c then done;
	}
	view loopView : StandardViewDefinitions::InterconnectionView { expose Loop; }
	view machineView : StandardViewDefinitions::StateTransitionView { expose Machine; }
	view driveView : StandardViewDefinitions::ActionFlowView { expose Drive; }
}
`

// A binding is an interconnection edge like a connection, drawn undirected and
// steered by the Route stated about it.
func TestBindingIsAnInterconnectionEdge(t *testing.T) {
	model := `package Bound {
	private import DiagramLayout::*;
	part def Pump { port level; }
	part def Tank { port level; }
	part def Loop {
		part pump : Pump;
		part tank : Tank;
		binding 'pump.level = tank.level' bind pump.level = tank.level;
	}
	view loopView : StandardViewDefinitions::InterconnectionView {
		expose Loop;
		metadata Route about Loop::'pump.level = tank.level' { points = (10, 20, 30, 40); }
	}
}
`
	r, idx := loadSources(t, []string{"bound.sysml"}, [][]byte{[]byte(model)})
	rendering, err := r.Render(lookup(t, idx, "Bound::loopView"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(rendering.Edges) != 1 || rendering.Edges[0].Kind != EdgeBinding || len(rendering.Edges[0].Route) != 2 {
		t.Fatalf("edges = %+v, want one routed binding", rendering.Edges)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	want := `"n1" -> "n2" [label="'pump.level = tank.level'", arrowhead=none, pos="10,-20 10,-20 30,-40 30,-40"];`
	if !strings.Contains(dot, want) {
		t.Errorf("DOT lacks %q:\n%s", want, dot)
	}
}

// A trigger names its signal or operation by the name it ends in, as a type is
// headed, however far the source qualifies it; its payload name and call
// arguments are kept, and time and change events keep their written text.
func TestTriggerLabelsHeadTheirSignalByItsEndName(t *testing.T) {
	model := `package Triggers {
	package Signals { package 'APS Internal' { attribute def 'Go Now'; attribute def Halt; attribute def 'Go::Now'; } }
	action def setSpeed { in value : ScalarValues::Real; }
	action def halt;
	state def Machine {
		entry; then idle;
		state idle;
		state moving;
		state stopped;
		state parked;
		transition first idle accept Triggers::Signals::'APS Internal'::'Go Now' then moving;
		transition first moving accept msg : Signals::'APS Internal'::Halt then stopped;
		transition first stopped accept Triggers::setSpeed(value) then moving;
		transition first moving accept after 5 then idle;
		transition first idle accept Signals::'APS Internal'::'Go::Now' then parked;
		transition first parked accept Triggers::halt() then stopped;
	}
	view machineView : StandardViewDefinitions::StateTransitionView { expose Machine; }
}
`
	r, idx := loadSources(t, []string{"triggers.sysml"}, [][]byte{[]byte(model)})
	rendering, err := r.Render(lookup(t, idx, "Triggers::machineView"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	text := rendering.Text()
	for _, want := range []string{
		"idle -> moving: accept 'Go Now'",
		"moving -> stopped: accept msg : Halt",
		"stopped -> moving: accept setSpeed(value)",
		"moving -> idle: after 5",
		"idle -> parked: accept 'Go::Now'",
		"parked -> stopped: accept halt()",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
	for _, edge := range rendering.Edges {
		if strings.Contains(edge.Label, "Triggers::") || strings.Contains(edge.Label, "Signals::") {
			t.Errorf("a trigger label keeps its qualification: %q", edge.Label)
		}
	}
}

// A name labels an edge only when the edge has no text of its own: a trigger, guard,
// pin or payload takes the label and the name is left out, given or synthesized.
func TestEdgeLabelsYieldToTheEdgesOwnText(t *testing.T) {
	r, idx := loadSources(t, []string{"labels.sysml"}, [][]byte{[]byte(edgeLabelModel)})
	cases := []struct {
		view string
		want []string
		skip []string
	}{
		{"Labels::machineView",
			[]string{"off -> on: 'off then on'", "on -> idle: accept Sig"},
			[]string{"'on accept Sig then idle'"}},
		{"Labels::driveView",
			[]string{"start -> a: 'start to a'", "a -> b: [true]", "b -> c: 'b to c'", "a => b: o to i"},
			[]string{"'a to b'", "'a.o to b.i'"}},
		{"Labels::loopView",
			[]string{"pump -- tank: supply", "pump == tank: 'pump.level = tank.level'", "pump => tank: of Water"},
			[]string{"'pump.outlet to tank.inlet'"}},
	}
	for _, tc := range cases {
		rendering, err := r.Render(lookup(t, idx, tc.view))
		if err != nil {
			t.Fatalf("render %s: %v", tc.view, err)
		}
		text := rendering.Text()
		for _, want := range tc.want {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q:\n%s", tc.view, want, text)
			}
		}
		for _, skip := range tc.skip {
			if strings.Contains(text, skip) {
				t.Errorf("%s labels an edge with its name beside its own text %q:\n%s", tc.view, skip, text)
			}
		}
	}
}
