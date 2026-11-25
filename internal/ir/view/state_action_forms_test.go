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
			do action pump { out pumped : Target; }
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
	var prepare, pump *Node
	for _, child := range active.Children {
		switch child.Kind {
		case entryActionKind:
			prepare = child
		case doActionKind:
			pump = child
		}
	}
	if prepare == nil || prepare.Name != "prepare" || pump == nil || pump.Name != "pump" {
		t.Fatalf("active's children %v lack the «entry action» prepare and «do action» pump", nodeNames(active.Children))
	}
	// A pin of the action's definition is inherited; one the usage declares is its own.
	wantEqual(t, "prepare's pins", pinTexts(prepare), []string{"in target"})
	if !prepare.Ports[0].Inherited {
		t.Errorf("prepare's target pin comes from Prepare, so is inherited: %+v", prepare.Ports[0])
	}
	wantEqual(t, "pump's pins", pinTexts(pump), []string{"out pumped"})
	if pump.Ports[0].Inherited {
		t.Errorf("pump declares its pumped pin, so it is its own: %+v", pump.Ports[0])
	}
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
