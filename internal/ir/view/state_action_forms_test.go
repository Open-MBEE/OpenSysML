package view

import (
	"strings"
	"testing"
)

// A composite state whose only members besides its actions are pseudostates
// still draws its entry action as a node under the `state transition` heading,
// and the action's declared parameters are its pins.
func TestPseudostateOnlyStateDrawsItsActionNodesWithPins(t *testing.T) {
	rendering := renderSource(t, "Diagrams::machine", `package Model {
	part def Target;
	action def Prepare { in target : Target; }
	state def Machine {
		entry; then active;
		state active {
			entry action prepare : Prepare;
			choice decide;
		}
	}
}
package Diagrams {
	private import StandardViewDefinitions::*;
	view machine : StateTransitionView {
		expose Model::Machine;
	}
}`)
	active := findNode(t, rendering.Roots, "active")
	var prepare *Node
	for _, child := range active.Children {
		if child.Kind == entryActionKind {
			prepare = child
		}
	}
	if prepare == nil || prepare.Name != "prepare" {
		t.Fatalf("active's children %v lack the «entry action» prepare", nodeNames(active.Children))
	}
	wantEqual(t, "pins", pinTexts(prepare), []string{"in target"})
	if !hasCompartment(active, stateTransitionCompartment) {
		t.Errorf("active's compartments %+v lack %q", active.Compartments, stateTransitionCompartment)
	}
	wantNoNotices(t, rendering)
}

func hasCompartment(node *Node, name string) bool {
	for _, compartment := range node.Compartments {
		if compartment.Name == name {
			return true
		}
	}
	return false
}

// A pin a Layout places on another side of its owner than its direction's
// default keeps pointing into the node: an input on the right border reads `←`.
func TestDrawioPlacedPinArrowFollowsItsSide(t *testing.T) {
	rendering := renderSource(t, "Diagrams::placed", `package Model {
	part def Target;
	action def Prepare { in target : Target; }
}
package Diagrams {
	private import Views::*;
	view placed {
		expose Model::Prepare;
		render asTreeDiagram;
		metadata DiagramLayout::Layout about Model::Prepare { x = 0; y = 0; width = 200; height = 100; }
		metadata DiagramLayout::Layout about Model::Prepare::target { x = 194; y = 44; }
	}
}`)
	doc, err := rendering.Drawio()
	if err != nil {
		t.Fatalf("drawio: %v", err)
	}
	if !strings.Contains(doc, `value="←"`) || strings.Contains(doc, `value="→"`) {
		t.Errorf("the input placed on the right border should point into the node (←):\n%s", doc)
	}
}
