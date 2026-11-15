package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const chainEndParts = `
	part def Vehicle {
		action close;
		action lock;
	}
`

// impliedChainSteps are the steps of graph a chained succession end implied.
func impliedChainSteps(graph *ActionGraph) []*ast.PerformActionNode {
	var steps []*ast.PerformActionNode
	for _, node := range graph.Nodes {
		if step, ok := node.(*ast.PerformActionNode); ok {
			steps = append(steps, step)
		}
	}
	return steps
}

// A chain end through a part implies one step per path, in either notation for
// an end and however many successions name it, performing the chain by reference.
func TestActionChainEndImpliesOneStepPerPath(t *testing.T) {
	graph := scopedActionGraph(t, chainEndParts+`
		action def A {
			part v : Vehicle;
			action a;
			first start then v.close;
			succession first v.close then a;
			first a then v.lock;
			first v.lock then done;
		}
	`, "A")
	steps := impliedChainSteps(graph)
	if len(steps) != 2 {
		t.Fatalf("implied steps = %d, want one each for v.close and v.lock", len(steps))
	}
	for i, want := range []string{"v.close", "v.lock"} {
		target, isChain := steps[i].ActionRef.(*ast.FeatureChainExpr)
		if !isChain || FeaturePath(target) != want {
			t.Errorf("step %d performs %T %q, want the feature chain %q", i, steps[i].ActionRef, FeaturePath(steps[i].ActionRef), want)
		}
		if kind := graph.Kinds[graph.ID[steps[i]]]; kind != program.NodePerform {
			t.Errorf("step %d kind = %s, want a perform node", i, kind)
		}
		if nodeHasName(steps[i]) {
			t.Errorf("step %d answers to a name, want it reachable only through its chain", i)
		}
	}
	closeStep, lockStep := ast.Node(steps[0]), ast.Node(steps[1])
	a := nodeNamed(t, graph, "a")
	if edges := graph.Edges[namedActionNode(t, graph, "start")]; len(edges) != 1 || edges[0].Target != closeStep {
		t.Errorf("start edges = %v, want [v.close]", edges)
	}
	if edges := graph.Edges[closeStep]; len(edges) != 1 || edges[0].Target != a {
		t.Errorf("v.close edges = %v, want [a]", edges)
	}
	if edges := graph.Edges[a]; len(edges) != 1 || edges[0].Target != lockStep {
		t.Errorf("a edges = %v, want [v.lock]", edges)
	}
}

// A step the body states to perform the chain is the occurrence the end names:
// no step is implied, and several such steps are each related.
func TestActionChainEndJoinsStatedPerformers(t *testing.T) {
	graph := scopedActionGraph(t, chainEndParts+`
		action def A {
			part v : Vehicle;
			perform action c1 ::> v.close;
			perform action c2 ::> v.close;
			perform v.lock;
			action a;
			first a then v.close;
			first v.close then v.lock;
		}
	`, "A")
	if implied := impliedChainSteps(graph); len(implied) != 0 {
		t.Fatalf("implied steps = %d, want none beside the stated performers", len(implied))
	}
	c1, c2 := nodeNamed(t, graph, "c1"), nodeNamed(t, graph, "c2")
	var lock ast.Node
	for _, node := range graph.Nodes {
		if performsPath(node, []string{"v", "lock"}) {
			lock = node
		}
	}
	targets := map[ast.Node]bool{}
	for _, edge := range graph.Edges[nodeNamed(t, graph, "a")] {
		targets[edge.Target] = true
	}
	if len(targets) != 2 || !targets[c1] || !targets[c2] {
		t.Errorf("a leads to %d steps, want c1 and c2", len(targets))
	}
	for _, source := range []ast.Node{c1, c2} {
		if edges := graph.Edges[source]; len(edges) != 1 || edges[0].Target != lock {
			t.Errorf("%s edges = %v, want [lock]", getNodeName(source), edges)
		}
	}
}

// A chain whose first segment names a node of the flow stays attached to that
// node in both notations for an end, as before.
func TestActionChainEndRootedAtANodeNamesTheNode(t *testing.T) {
	graph := scopedActionGraph(t, `
		action def A {
			action source { attribute pin = 0; }
			action target;
			first start then source.pin;
			succession first source.pin then target;
		}
	`, "A")
	source := nodeNamed(t, graph, "source")
	if steps := impliedChainSteps(graph); len(steps) != 0 {
		t.Fatalf("implied steps = %d, want none", len(steps))
	}
	if edges := graph.Edges[namedActionNode(t, graph, "start")]; len(edges) != 1 || edges[0].Target != source {
		t.Errorf("start edges = %v, want [source]", edges)
	}
	if edges := graph.Edges[source]; len(edges) != 1 || edges[0].Target != nodeNamed(t, graph, "target") {
		t.Errorf("source edges = %v, want [target]", edges)
	}
}

func TestClassifyChainEnd(t *testing.T) {
	graph := scopedActionGraph(t, `
		part def Vehicle {
			action close;
			event occurrence opened;
			attribute n = 0;
		}
		action def A {
			part v : Vehicle;
			action a { action inner; }
			first start then a;
		}
	`, "A")
	chain := func(segments ...string) ast.Node {
		qn := ast.QualifiedNameOf(segments[0])
		for _, segment := range segments[1:] {
			qn.Parts = append(qn.Parts, ast.NameSegment{Text: segment, Chained: true})
		}
		return qn
	}
	for name, c := range map[string]struct {
		ref  ast.Node
		want ChainEnd
	}{
		"action":             {chain("v", "close"), ChainEndAction},
		"occurrence":         {chain("v", "opened"), ChainEndOccurrence},
		"attribute":          {chain("v", "n"), ChainEndValue},
		"no such feature":    {chain("v", "missing"), ChainEndNone},
		"rooted at a node":   {chain("a", "inner"), ChainEndNone},
		"plain name":         {chain("v"), ChainEndNone},
		"qualified name":     {ast.QualifiedNameOf("v", "close"), ChainEndNone},
		"no end":             {nil, ChainEndNone},
		"absent name":        {(*ast.QualifiedName)(nil), ChainEndNone},
		"expression form":    {chainTarget(chain("v", "close")), ChainEndAction},
		"expression to none": {chainTarget(chain("v", "missing")), ChainEndNone},
	} {
		t.Run(name, func(t *testing.T) {
			got, sym := ClassifyChainEnd(graph.Nodes, graph.Scope, c.ref)
			if got != c.want || (sym != nil) != (c.want != ChainEndNone) {
				t.Fatalf("ClassifyChainEnd = %d (symbol %v), want %d", got, sym != nil, c.want)
			}
		})
	}
}

// A step a general states to perform the chain is the end of a succession the
// specialization adds, though it joins the flow after that succession is lowered.
func TestActionChainEndJoinsInheritedPerformer(t *testing.T) {
	graph := scopedActionGraph(t, chainEndParts+`
		action def Base {
			part v : Vehicle;
			perform action c ::> v.close;
		}
		action def Derived :> Base {
			action a;
			first v.close then a;
		}
	`, "Derived")
	if implied := impliedChainSteps(graph); len(implied) != 0 {
		t.Fatalf("implied steps = %d, want the inherited performer to be the end", len(implied))
	}
	c := nodeNamed(t, graph, "c")
	if edges := graph.Edges[c]; len(edges) != 1 || edges[0].Target != nodeNamed(t, graph, "a") {
		t.Fatalf("c edges = %v, want [a]", edges)
	}
	count := 0
	for _, node := range graph.Nodes {
		if node == c {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("c is a node %d times, want once", count)
	}
}

// An implied step answers to no name, and chains differing only in where a
// quoted name holds the dot are different chains.
func TestActionChainEndImpliedStepsStayApart(t *testing.T) {
	graph := scopedActionGraph(t, chainEndParts+`
		part def Lock { action lock; }
		part def Fleet { part lead : Vehicle; }
		action def A {
			part v : Vehicle;
			part close : Lock;
			part fleet : Fleet;
			part 'fleet.lead' : Vehicle;
			first v.close then close.lock;
			first fleet.lead.close then 'fleet.lead'.close;
		}
	`, "A")
	steps := impliedChainSteps(graph)
	if len(steps) != 4 {
		t.Fatalf("implied steps = %d, want four", len(steps))
	}
	for i, pair := range [][2]int{{0, 1}, {2, 3}} {
		source, target := ast.Node(steps[pair[0]]), ast.Node(steps[pair[1]])
		if edges := graph.Edges[source]; len(edges) != 1 || edges[0].Target != target {
			t.Errorf("succession %d leads to %v, want the step of its own target chain", i, edges)
		}
	}
}

// A chain through a part named as a flow marker is no marker in an inherited succession.
func TestActionChainEndInheritedThroughAMarkerNamedPart(t *testing.T) {
	graph := scopedActionGraph(t, chainEndParts+`
		action def Base {
			part done : Vehicle;
			part start : Vehicle;
			action a;
			first start.close then a;
			first a then done.close;
		}
		action def Derived :> Base;
	`, "Derived")
	steps := impliedChainSteps(graph)
	if len(steps) != 2 || len(graph.Finals) != 0 || graph.Initial != nil && graph.Initial != ast.Node(steps[0]) {
		t.Fatalf("implied steps = %d, finals = %d, initial = %T; want two steps and no marker", len(steps), len(graph.Finals), graph.Initial)
	}
	a := nodeNamed(t, graph, "a")
	if edges := graph.Edges[steps[0]]; len(edges) != 1 || edges[0].Target != a {
		t.Errorf("start.close edges = %v, want [a]", edges)
	}
	if edges := graph.Edges[a]; len(edges) != 1 || edges[0].Target != ast.Node(steps[1]) {
		t.Errorf("a edges = %v, want [done.close]", edges)
	}
}

// A performer a nearer general redefines without restating the reference is
// still the occurrence the chain names: the redefining step is the end.
func TestActionChainEndJoinsRedefinedInheritedPerformer(t *testing.T) {
	graph := scopedActionGraph(t, chainEndParts+`
		action def Base {
			part v : Vehicle;
			perform action c ::> v.close;
		}
		action def Mid :> Base {
			action c2 :>> c;
		}
		action def Derived :> Mid {
			action a;
			first v.close then a;
		}
	`, "Derived")
	if implied := impliedChainSteps(graph); len(implied) != 0 {
		t.Fatalf("implied steps = %d, want the redefining step to be the end", len(implied))
	}
	if edges := graph.Edges[nodeNamed(t, graph, "c2")]; len(edges) != 1 || edges[0].Target != nodeNamed(t, graph, "a") {
		t.Fatalf("c2 edges = %v, want [a]", edges)
	}
	referrer := graph.Performs[nodeNamed(t, graph, "c2")].Referrer
	if referrer == nil || !performsPath(referrer, []string{"v", "close"}) {
		t.Fatalf("c2 performs through %v, want the reference of the step it redefines", referrer)
	}
}
