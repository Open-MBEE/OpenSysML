package view

import (
	"strings"
	"testing"
)

// nodeAt follows a path of node names from the roots and returns the node it
// ends at, nil when some step is not drawn.
func nodeAt(roots []*Node, path ...string) *Node {
	nodes := roots
	var found *Node
	for _, name := range path {
		found = nil
		for _, node := range nodes {
			if node.Name == name {
				found = node
				break
			}
		}
		if found == nil {
			return nil
		}
		nodes = found.Children
	}
	return found
}

func childNames(node *Node) []string {
	if node == nil {
		return nil
	}
	out := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		out = append(out, child.Name)
	}
	return out
}

// A bodiless usage shows the features it inherits from its definition, a typed
// nested usage expands through its own type, a redefining feature stands in
// for the one it redefines, and a self-typed part is bounded by the depth.
func TestInterconnectionExpandsUsagesThroughTheirTypes(t *testing.T) {
	rendering := render(t, "typed-usage-inherited.sysml", "TypedUsageViews::rocketLinks")
	rocket := nodeAt(rendering.Roots, "TypedUsageViews::rocket")
	if rocket == nil {
		t.Fatalf("rocket is not drawn; roots: %v", nodeNames(rendering.Roots))
	}
	for _, want := range []string{"mass", "stage1", "stage2", "subcomponents"} {
		if nodeAt(rocket.Children, want) == nil {
			t.Errorf("rocket lacks %s inherited from Vehicle; children: %v", want, childNames(rocket))
		}
	}
	if !hasPort(rocket, "telemetry") {
		t.Errorf("rocket lacks the port telemetry inherited from Vehicle; ports: %v", rocket.Ports)
	}
	stage1 := nodeAt(rocket.Children, "stage1")
	if stage1 == nil {
		t.Fatalf("stage1 is not drawn under rocket; children: %v", childNames(rocket))
	}
	for _, want := range []string{"propellantMass", "engines", "subcomponents"} {
		if nodeAt(stage1.Children, want) == nil {
			t.Errorf("stage1 : Stage lacks %s; children: %v", want, childNames(stage1))
		}
	}
	if nodeAt(stage1.Children, "thrust") != nil {
		t.Errorf("stage1 draws Engine's thrust as its own; children: %v", childNames(stage1))
	}
	if engines := nodeAt(stage1.Children, "engines"); engines != nil {
		if nodeAt(engines.Children, "thrust") == nil || nodeAt(engines.Children, "mass") == nil {
			t.Errorf("engines : Engine lacks its inherited attributes; children: %v", childNames(engines))
		}
	}
	// Vehicle redefines mass, stage2 redefines propellantMass and Stage
	// redefines subcomponents: each is drawn once, the redefining feature
	// standing in for the inherited one.
	for _, owner := range []*Node{rocket, stage1, nodeAt(rocket.Children, "stage2")} {
		for _, name := range []string{"mass", "propellantMass", "subcomponents"} {
			if n := len(filterNames(childNames(owner), name)); n > 1 {
				t.Errorf("%s draws %s %d times, want at most once; children: %v", owner.Name, name, n, childNames(owner))
			}
		}
	}
	if strings.Count(rendering.Text(), "already shown") != 0 {
		t.Errorf("an inherited feature is noted as already shown:\n%s", rendering.Text())
	}
	var connects int
	for _, edge := range rendering.Edges {
		if edge.Kind == EdgeConnection {
			connects++
		}
	}
	if connects != 1 {
		t.Errorf("connections = %d, want the one Vehicle declares between stage1.status and telemetry", connects)
	}
}

func hasPort(node *Node, name string) bool {
	for _, port := range node.Ports {
		if port.Name == name {
			return true
		}
	}
	return false
}

func filterNames(names []string, want string) []string {
	var out []string
	for _, name := range names {
		if name == want {
			out = append(out, name)
		}
	}
	return out
}

// A tree of a definition expands a typed nested usage through its type.
func TestTreeExpandsNestedUsageThroughItsType(t *testing.T) {
	rendering := render(t, "typed-usage-inherited.sysml", "TypedUsageViews::vehicleTree")
	stage1 := nodeAt(rendering.Roots, "TypedUsageViews::Vehicle", "stage1")
	if stage1 == nil {
		t.Fatalf("Vehicle::stage1 is not drawn; roots: %v", nodeNames(rendering.Roots))
	}
	for _, want := range []string{"propellantMass", "engines"} {
		if nodeAt(stage1.Children, want) == nil {
			t.Errorf("stage1 : Stage lacks %s; children: %v", want, childNames(stage1))
		}
	}
}
