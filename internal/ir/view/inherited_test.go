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

const inheritedEdgeCases = `package Cases {
	private import Views::*;
	part def Base {
		attribute x;
		port p;
		connect x to p;
	}
	part def Derived :> Base {
		part inner : Base {
			attribute :>> x;
		}
		attribute :>> x;
	}
	part instance : Derived;
	view instanceLinks {
		expose instance;
		render asInterconnectionDiagram;
	}

	part def Stage {
		attribute propellantMass;
	}
	part def Vehicle {
		part stage1 : Stage;
		part stage2 : Stage {
			attribute :>> propellantMass = 2;
		}
	}
	view vehicleTree {
		expose Vehicle;
		render asTreeDiagram;
	}

	part def Box {
		part next : Box {
			attribute serial;
		}
	}
	view boxTree {
		expose Box;
		render asTreeDiagram;
	}
}
`

// A connector a definition declares between its own features ends, in a usage
// of a type redefining one of them, at the redefining feature of that usage —
// not at a nested part's redefinition of the same feature drawn earlier.
func TestInheritedConnectorEndsAtTheUsagesOwnRedefinition(t *testing.T) {
	r := renderSource(t, "Cases::instanceLinks", inheritedEdgeCases)
	instance := nodeAt(r.Roots, "Cases::instance")
	if instance == nil {
		t.Fatalf("instance is not drawn; roots: %v", nodeNames(r.Roots))
	}
	x, inner := nodeAt(instance.Children, "x"), nodeAt(instance.Children, "inner")
	if x == nil || inner == nil || nodeAt(inner.Children, "x") == nil {
		t.Fatalf("instance lacks x or inner.x; children: %v", childNames(instance))
	}
	var toX int
	for _, edge := range r.Edges {
		if edge.Kind != EdgeConnection {
			continue
		}
		if edge.From == x.ID || edge.To == x.ID {
			toX++
		}
		if other := nodeAt(inner.Children, "x"); edge.From == other.ID || edge.To == other.ID {
			if edge.From != inner.ID && edge.To != inner.ID && (edge.FromPort == "" || edge.ToPort == "") {
				t.Errorf("edge %+v joins inner's x from outside inner", edge)
			}
		}
	}
	if toX != 1 {
		t.Errorf("connections at instance's own x = %d, want the one Base declares; edges: %+v", toX, r.Edges)
	}
}

// A redefinition arrow joins declarations: stage2's propellantMass redefines
// Stage's, which is drawn only as inherited into stage1 and stage2, so no arrow
// is drawn from one stage's copy to the other's.
func TestTreeArrowsDoNotCrossUsagesOfOneType(t *testing.T) {
	r := renderSource(t, "Cases::vehicleTree", inheritedEdgeCases)
	for _, edge := range r.Edges {
		if edge.Kind == EdgeSpecialization {
			t.Errorf("edge %+v joins two usages' copies of Stage::propellantMass", edge)
		}
	}
}

// A self-typed part shows what it declares itself under the depth bound; only
// the members it would inherit from the type being expanded above it are left out.
func TestRecursiveUsageKeepsItsDeclaredMembers(t *testing.T) {
	r := renderSource(t, "Cases::boxTree", inheritedEdgeCases)
	next := nodeAt(r.Roots, "Cases::Box", "next")
	if next == nil {
		t.Fatalf("Box::next is not drawn; roots: %v", nodeNames(r.Roots))
	}
	if nodeAt(next.Children, "serial") == nil {
		t.Errorf("next lacks the serial it declares; children: %v, detail %q", childNames(next), next.Detail)
	}
	if nodeAt(next.Children, "next") != nil {
		t.Errorf("next unfolds Box again; children: %v", childNames(next))
	}
	if !strings.Contains(next.Detail, "recursive through Box") {
		t.Errorf("detail = %q, want the recursion noted", next.Detail)
	}
}

// A redefining usage that declares no typing is headed with the type it inherits
// from the feature it redefines; one redefining an untyped feature stays untyped.
func TestRedefiningUsageHeadsWithInheritedType(t *testing.T) {
	rendering := render(t, "typed-usage-inherited.sysml", "TypedUsageViews::rocketLinks")
	rocket := nodeAt(rendering.Roots, "TypedUsageViews::rocket")
	if rocket == nil {
		t.Fatalf("rocket is not drawn; roots: %v", nodeNames(rendering.Roots))
	}
	for _, path := range [][]string{{"subcomponents"}, {"stage1", "subcomponents"}} {
		node := nodeAt(rocket.Children, path...)
		if node == nil {
			t.Fatalf("%v is not drawn under rocket", path)
		}
		if node.Type != "Component" || strings.Join(node.Typings, ",") != "TypedUsageViews::Component" {
			t.Errorf("%v: type %q typings %v, want Component inherited from Component::subcomponents", path, node.Type, node.Typings)
		}
	}
	if mass := nodeAt(rocket.Children, "mass"); mass == nil || mass.Type != "" || len(mass.Typings) != 0 {
		t.Errorf("mass redefines an untyped attribute, want no type; got %+v", mass)
	}
}

// A usage redefining typed features is headed with every type they declare, each element
// once in redefinition order, through untyped chains, the same in every rendering form.
func TestRedefiningUsageHeadsWithEveryInheritedType(t *testing.T) {
	rendering := render(t, "redefining-heads.sysml", "RedefiningHeads::heads")
	d := nodeAt(rendering.Roots, "RedefiningHeads::d")
	if d == nil {
		t.Fatalf("d is not drawn; roots: %v", nodeNames(rendering.Roots))
	}
	want := map[string][2]string{
		"both":   {"A, B", "RedefiningHeads::A,RedefiningHeads::B"},
		"single": {"A", "RedefiningHeads::A"},
		"pair":   {"A", "RedefiningHeads::A"},
		"lbrb":   {"A, B", "RedefiningHeads::A,RedefiningHeads::B"},
		"q":      {"Q::A, A", "RedefiningHeads::Q::A,RedefiningHeads::A"},
	}
	for name, typing := range want {
		node := nodeAt(d.Children, name)
		if node == nil {
			t.Fatalf("%s is not drawn under d; children: %v", name, childNames(d))
		}
		if node.Type != typing[0] || strings.Join(node.Typings, ",") != typing[1] {
			t.Errorf("interconnection %s: type %q typings %v, want %q %q", name, node.Type, node.Typings, typing[0], typing[1])
		}
	}
	tree := render(t, "redefining-heads.sysml", "RedefiningHeads::headsTree")
	further := nodeAt(tree.Roots, "RedefiningHeads::Further")
	if further == nil {
		t.Fatalf("Further is not drawn in the tree; roots: %v", nodeNames(tree.Roots))
	}
	for name, typing := range want {
		if node := nodeAt(further.Children, name); node == nil || node.Type != typing[0] {
			t.Errorf("tree %s = %+v, want type %q", name, node, typing[0])
		}
	}
	table := render(t, "redefining-heads.sysml", "RedefiningHeads::headsTable")
	typed := map[string]string{}
	for _, row := range table.Rows {
		typed[row[0]] = row[2]
	}
	for name, typing := range want {
		if typed[name] != typing[0] {
			t.Errorf("table %s type = %q, want %q (rows %v)", name, typed[name], typing[0], table.Rows)
		}
	}
}
