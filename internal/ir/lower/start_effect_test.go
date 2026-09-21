package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A perform naming the start of a behavior an object holds lowers to a start
// effect whose target is the behavior, whether written as a statement of a node's
// body, as a node of the flow, or as a named performed action referring to it.
func TestPerformOfStartLowersToStartEffect(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			first start;
			action make { out target : Runner; }
			action kick {
				in target : Runner;
				perform target.beh.start;
			}
			perform make.target.beh.start;
			action third { in target : Runner; perform action kick2 references target.beh.start; }
			done;
			succession first start then make;
			succession first make then kick;
			succession first kick then third;
			succession first third then done;
		}
	`)

	kick := nodeNamed(t, graph, "kick")
	body := graph.Bodies[kick]
	if len(body) != 1 {
		t.Fatalf("kick lowered to %d statements, want 1: %#v", len(body), body)
	}
	assertStart(t, body[0], "target.beh")

	third := nodeNamed(t, graph, "third")
	body = graph.Bodies[third]
	if len(body) != 1 {
		t.Fatalf("third lowered to %d statements, want 1: %#v", len(body), body)
	}
	assertStart(t, body[0], "target.beh")

	var flowStart *Effect
	for _, node := range graph.Nodes {
		if node == kick || node == third {
			continue
		}
		if effect, ok := graph.StartUsage(node); ok {
			flowStart = &effect
		}
	}
	if flowStart == nil {
		t.Fatal("the perform node of the flow lowered to no start effect")
	}
	assertStart(t, *flowStart, "make.target.beh")
}

// A perform naming a behavior itself, with no start, stays a perform: the
// behavior is performed to completion as a step of the body.
func TestPerformOfBehaviorStaysPerform(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			first start;
			action kick {
				in target : Runner;
				perform target.beh;
				perform start;
			}
			done;
			succession first start then kick;
			succession first kick then done;
		}
	`)
	body := graph.Bodies[nodeNamed(t, graph, "kick")]
	if len(body) != 2 {
		t.Fatalf("kick lowered to %d statements, want 2: %#v", len(body), body)
	}
	for i, stmt := range body {
		effect, ok := stmt.(Effect)
		if !ok || effect.Kind != EffectPerform {
			t.Errorf("statement %d = %#v, want a perform effect", i, stmt)
		}
	}
}

// A `start` the part's type declares as an action of its own is performed; the
// behavior a part holds is started; an attribute's is left to the runtime to refuse.
func TestPerformOfDeclaredStartActionStaysPerform(t *testing.T) {
	graph := scopedActionGraph(t, `
		part def Vehicle {
			attribute n : Integer;
			action def Launch;
			action start : Launch;
			action def Run;
			action run : Run;
		}
		action def Test {
			action own { in v : Vehicle; perform v.start; }
			action held { in v : Vehicle; perform v.run.start; }
			action attr { in v : Vehicle; perform v.n.start; }
			first start then own; first own then held; first held then attr; first attr then done;
		}
	`, "Test")

	own := graph.Bodies[nodeNamed(t, graph, "own")]
	if len(own) != 1 {
		t.Fatalf("own lowered to %d statements, want 1: %#v", len(own), own)
	}
	if effect, ok := own[0].(Effect); !ok || effect.Kind != EffectPerform {
		t.Errorf("perform v.start, start an action Vehicle declares, lowered to %#v, want a perform effect", own[0])
	}

	held := graph.Bodies[nodeNamed(t, graph, "held")]
	if len(held) != 1 {
		t.Fatalf("held lowered to %d statements, want 1: %#v", len(held), held)
	}
	assertStart(t, held[0], "v.run")

	attr := graph.Bodies[nodeNamed(t, graph, "attr")]
	if len(attr) != 1 {
		t.Fatalf("attr lowered to %d statements, want 1: %#v", len(attr), attr)
	}
	assertStart(t, attr[0], "v.n")
}

func assertStart(t *testing.T, stmt Statement, target string) {
	t.Helper()
	effect, ok := stmt.(Effect)
	if !ok || effect.Kind != EffectStart {
		t.Fatalf("statement = %#v, want a start effect", stmt)
	}
	if got := exprText(effect.Target); got != target {
		t.Errorf("start target = %q, want %q", got, target)
	}
	chain, ok := effect.TargetExpr.(*ast.FeatureChainExpr)
	if !ok || chain.Operand != effect.Target {
		t.Errorf("start expression = %#v, want the chain reaching start through the target", effect.TargetExpr)
	}
}

// exprText spells a reference expression as written, dotted.
func exprText(node ast.Node) string {
	switch n := node.(type) {
	case *ast.FeatureChainExpr:
		return exprText(n.Operand) + "." + ast.QualifiedText(n.Member)
	case *ast.QualifiedName:
		return ast.QualifiedText(n)
	case *ast.FeatureReference:
		return ast.QualifiedText(n.Name)
	default:
		return ""
	}
}
