package view

import (
	"strings"
	"testing"
)

// inheritedMembers draws one declaration under several inheritors: Pump's
// waterOut under pump and sensor, Operate's start under Operate and run.
const inheritedMembers = `package Plant {
	part def Pump { item waterOut; }
	part def Loop {
		part pump : Pump;
		part sensor : Pump;
	}
	action def Operate {
		first start;
		action step;
		succession first start then step;
	}
	use case def Inspection { perform action run : Operate; }
	package Diagrams {
		private import Views::*;
		private import OpenSysMLRenderings::*;
		view loopView {
			expose Loop;
			render asInterconnectionDiagram;
			metadata DiagramLayout::Layout about Loop::pump::waterOut { x = 10; y = 20; }
			metadata DiagramLayout::Layout about Loop::sensor::waterOut { x = 30; y = 40; }
			metadata DiagramLayout::Style about Loop::sensor::waterOut { fill = "#ffcc00"; }
		}
		view treeView {
			expose Loop;
			render asTreeDiagram;
			metadata DiagramLayout::Layout about Loop::sensor::waterOut { x = 30; y = 40; }
		}
		view mixedView {
			expose Plant;
			render asMixedDiagram;
			metadata DiagramLayout::Layout about Operate::start { x = 5; y = 6; }
		}
	}
}
`

func renderInline(t *testing.T, view string) *Rendering {
	t.Helper()
	r, idx := loadSources(t, []string{"inherited.sysml"}, [][]byte{[]byte(inheritedMembers)})
	rendering, err := r.Render(lookup(t, idx, view))
	if err != nil {
		t.Fatalf("render %s: %v", view, err)
	}
	return rendering
}

// childNamed is the child of the node named owner that is named name.
func childNamed(t *testing.T, roots []*Node, owner, name string) *Node {
	t.Helper()
	for _, child := range findNode(t, roots, owner).Children {
		if child.Name == name {
			return child
		}
	}
	t.Fatalf("no %s under %s", name, owner)
	return nil
}

// A member drawn under each inheritor is placed and coloured by the Layout
// and Style naming it as that inheritor's, not by another inheritor's.
func TestInheritedMemberIsPlacedPerInheritor(t *testing.T) {
	for _, view := range []string{"Plant::Diagrams::loopView", "Plant::Diagrams::treeView"} {
		rendering := renderInline(t, view)
		sensor := childNamed(t, rendering.Roots, "sensor", "waterOut")
		if sensor.Geometry == nil || sensor.Geometry.X != 30 || sensor.Geometry.Y != 40 {
			t.Errorf("%s: sensor's waterOut at %+v, want (30, 40)", view, sensor.Geometry)
		}
		pump := childNamed(t, rendering.Roots, "pump", "waterOut")
		if view == "Plant::Diagrams::loopView" {
			if pump.Geometry == nil || pump.Geometry.X != 10 || pump.Geometry.Y != 20 {
				t.Errorf("pump's waterOut at %+v, want (10, 20)", pump.Geometry)
			}
			if sensor.Style == nil || !strings.EqualFold(sensor.Style.Fill, "#ffcc00") || pump.Style != nil {
				t.Errorf("styles: sensor %+v, pump %+v; want sensor's alone filled", sensor.Style, pump.Style)
			}
		} else if pump.Geometry != nil {
			t.Errorf("tree: pump's waterOut at %+v, want unplaced", pump.Geometry)
		}
	}
}

// A Layout about the declaring body's start places that start where its body
// is drawn; a body inheriting the start does not grow one at the same place.
func TestInheritedStartIsNotAddedByTheDeclaringBodysLayout(t *testing.T) {
	rendering := renderInline(t, "Plant::Diagrams::mixedView")
	placed := false
	for _, child := range findNode(t, rendering.Roots, "Plant::Operate").Children {
		if child.Kind == "initial" && child.Geometry != nil && child.Geometry.X == 5 && child.Geometry.Y == 6 {
			placed = true
		}
	}
	if !placed {
		t.Errorf("Operate's start is not placed at (5, 6)")
	}
	for _, child := range findNode(t, rendering.Roots, "Plant::Inspection::run").Children {
		if child.Kind == "initial" {
			t.Errorf("run draws a start at %+v from Operate's Layout", child.Geometry)
		}
	}
}
