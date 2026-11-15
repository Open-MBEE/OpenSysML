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
func impliedChainSteps(graph *ActionGraph) []*ast.Usage {
	var steps []*ast.Usage
	for _, node := range graph.Nodes {
		if usage, ok := node.(*ast.Usage); ok && usage.Keyword == "perform" && usage.Ident.Name == "" {
			steps = append(steps, usage)
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
		rel := steps[i].Relationships[0]
		if rel.Kind != ast.RelReferences || FeaturePath(rel.Target) != want {
			t.Errorf("step %d performs %s %q, want a reference to %q", i, rel.Kind, FeaturePath(rel.Target), want)
		}
		if _, isChain := rel.Target.(*ast.FeatureChainExpr); !isChain {
			t.Errorf("step %d target = %T, want a feature chain", i, rel.Target)
		}
		if kind := graph.Kinds[graph.ID[steps[i]]]; kind != program.NodeExecution {
			t.Errorf("step %d kind = %s, want an execution node", i, kind)
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
	stated := impliedChainSteps(graph)
	if len(stated) != 1 {
		t.Fatalf("anonymous performs = %d, want only the stated `perform v.lock;`", len(stated))
	}
	c1, c2, lock := nodeNamed(t, graph, "c1"), nodeNamed(t, graph, "c2"), ast.Node(stated[0])
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
