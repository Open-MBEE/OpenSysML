package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessActionLocalConnection exercises a connection an action declares over
// its own parameters: each performance holds a connector of its own, attached to what it binds.
func TestRuntimeRobustnessActionLocalConnection(t *testing.T) {
	t.Run("each_performance_connects_its_arguments", testActionLocalConnectionPerPerformance)
	t.Run("one_connector_per_performance", testActionLocalConnectionReadTwice)
	t.Run("inherited_from_the_definition", testActionLocalConnectionInherited)
	t.Run("read_by_a_nested_action", testActionLocalConnectionNested)
	t.Run("nested_action_declaring_the_same_name", testActionLocalConnectionShadowed)
	t.Run("end_selecting_nothing", testActionLocalConnectionEndSelectingNothing)
}

const actionLocalConnectionFixture = `
	package test {
		part def Node;
		connection def Link { end left : Node; end right : Node; }
		action def Connect {
			in ref firstNode : Node;
			in ref secondNode : Node;
			connection link : Link connect firstNode to secondNode;
		}
		part def Network {
			part a : Node;
			part b : Node;
			ref links : Link [0..*];
			ref again : Link [0..*];
			action connectNodes {
				in ref firstNode : Node;
				in ref secondNode : Node;
				connection link : Link connect firstNode to secondNode;
				assign links := link;
			}
			action connectTwice {
				in ref firstNode : Node;
				in ref secondNode : Node;
				connection link : Link connect firstNode to secondNode;
				assign links := link;
				assign again := link;
			}
			action connectInherited : Connect {
				assign links := link;
			}
			action connectNested {
				in ref firstNode : Node;
				in ref secondNode : Node;
				connection link : Link connect firstNode to secondNode;
				action inner { assign links := link; }
				assign again := link;
			}
			action connectShadowed {
				in ref firstNode : Node;
				in ref secondNode : Node;
				connection link : Link connect firstNode to secondNode;
				assign links := link;
				action inner {
					in ref x : Node = secondNode;
					in ref y : Node = firstNode;
					connection link : Link connect x to y;
					assign again := link;
				}
			}
			action connectIndexed {
				in ref firstNode : Node;
				in ref others : Node [0..*];
				connection link : Link connect firstNode to others#(2);
				assign links := link;
			}
		}
	}
`

// actionLocalConnectionNetwork instantiates the fixture's Network and returns it with its two nodes.
func actionLocalConnectionNetwork(t *testing.T) (*Context, *Instance, Value, Value) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, actionLocalConnectionFixture))
	network := findSymbolByName(idx.DocumentRoot("<test>"), "Network", ast.DefPart)
	if network == nil {
		t.Fatal("Network not found")
	}
	inst := mustInstantiate(t, ctx, network)
	return ctx, inst, networkObject(t, ctx, inst, "a"), networkObject(t, ctx, inst, "b")
}

// networkObject is the one object inst holds under feature.
func networkObject(t *testing.T, ctx *Context, inst *Instance, feature string) Value {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, feature)
	if err != nil {
		t.Fatalf("%s: %v", feature, err)
	}
	held := elementsOf(fv.HeldValue())
	if len(held) != 1 || held[0].Kind != ValInstance {
		t.Fatalf("%s = %v, want one object", feature, fv.HeldValue())
	}
	return held[0]
}

// wantLinkEnds checks that the connector inst holds under feature attaches left and right.
func wantLinkEnds(t *testing.T, ctx *Context, inst *Instance, feature string, left, right Value) int64 {
	t.Helper()
	link := networkObject(t, ctx, inst, feature)
	conn := ctx.instances[link.Instance]
	if conn == nil || len(conn.Ends) != 2 {
		t.Fatalf("%s holds %v, want a connector of two ends", feature, link)
	}
	if !valueIdentical(conn.Ends[0].Value, left) || !valueIdentical(conn.Ends[1].Value, right) {
		t.Errorf("%s ends = %v, %v, want %v, %v", feature, conn.Ends[0].Value, conn.Ends[1].Value, left, right)
	}
	return link.Instance
}

// testActionLocalConnectionPerPerformance: a second performance bound to other
// arguments holds a connector of its own, not the first performance's.
func testActionLocalConnectionPerPerformance(t *testing.T) {
	ctx, inst, a, b := actionLocalConnectionNetwork(t)
	if _, err := ctx.invokeOperation(inst, "connectNodes", map[string]Value{"firstNode": a, "secondNode": b}); err != nil {
		t.Fatalf("connectNodes(a, b): %v", err)
	}
	first := wantLinkEnds(t, ctx, inst, "links", a, b)
	if _, err := ctx.invokeOperation(inst, "connectNodes", map[string]Value{"firstNode": b, "secondNode": a}); err != nil {
		t.Fatalf("connectNodes(b, a): %v", err)
	}
	if second := wantLinkEnds(t, ctx, inst, "links", b, a); second == first {
		t.Errorf("both performances hold connector #%d, want one each", first)
	}
}

// testActionLocalConnectionReadTwice: two reads in one performance name one connector.
func testActionLocalConnectionReadTwice(t *testing.T) {
	ctx, inst, a, b := actionLocalConnectionNetwork(t)
	if _, err := ctx.invokeOperation(inst, "connectTwice", map[string]Value{"firstNode": a, "secondNode": b}); err != nil {
		t.Fatalf("connectTwice(a, b): %v", err)
	}
	if first, second := wantLinkEnds(t, ctx, inst, "links", a, b), wantLinkEnds(t, ctx, inst, "again", a, b); first != second {
		t.Errorf("reads hold connectors #%d and #%d, want one", first, second)
	}
}

// testActionLocalConnectionInherited: a connection the action's definition declares is
// attached to what the performance of the usage binds.
func testActionLocalConnectionInherited(t *testing.T) {
	ctx, inst, a, b := actionLocalConnectionNetwork(t)
	if _, err := ctx.invokeOperation(inst, "connectInherited", map[string]Value{"firstNode": b, "secondNode": a}); err != nil {
		t.Fatalf("connectInherited(b, a): %v", err)
	}
	wantLinkEnds(t, ctx, inst, "links", b, a)
}

// testActionLocalConnectionNested: a nested action reads the connector of the performance around it.
func testActionLocalConnectionNested(t *testing.T) {
	ctx, inst, a, b := actionLocalConnectionNetwork(t)
	if _, err := ctx.invokeOperation(inst, "connectNested", map[string]Value{"firstNode": a, "secondNode": b}); err != nil {
		t.Fatalf("connectNested(a, b): %v", err)
	}
	if inner, outer := wantLinkEnds(t, ctx, inst, "links", a, b), wantLinkEnds(t, ctx, inst, "again", a, b); inner != outer {
		t.Errorf("nested and enclosing reads hold connectors #%d and #%d, want one", inner, outer)
	}
}

// testActionLocalConnectionShadowed: a nested action declaring a connection of the enclosing
// one's name reads its own connector, though the enclosing one was read first.
func testActionLocalConnectionShadowed(t *testing.T) {
	ctx, inst, a, b := actionLocalConnectionNetwork(t)
	if _, err := ctx.invokeOperation(inst, "connectShadowed", map[string]Value{"firstNode": a, "secondNode": b}); err != nil {
		t.Fatalf("connectShadowed(a, b): %v", err)
	}
	if outer, inner := wantLinkEnds(t, ctx, inst, "links", a, b), wantLinkEnds(t, ctx, inst, "again", b, a); outer == inner {
		t.Errorf("both actions hold connector #%d, want one each", outer)
	}
}

// testActionLocalConnectionEndSelectingNothing: an end indexing past what its parameter holds is
// refused as an end that cannot be attached, and no connector is left behind.
func testActionLocalConnectionEndSelectingNothing(t *testing.T) {
	ctx, inst, a, b := actionLocalConnectionNetwork(t)
	before := len(ctx.instances)
	_, err := ctx.invokeOperation(inst, "connectIndexed", map[string]Value{"firstNode": a, "others": b})
	if !errors.Is(err, errConnectorEnd) || !strings.Contains(err.Error(), `end "others#(2)"`) {
		t.Fatalf("error = %v, want errConnectorEnd naming others#(2)", err)
	}
	if len(ctx.instances) != before {
		t.Errorf("%d objects after the refused run, want %d", len(ctx.instances), before)
	}
	if fv, err := inst.GetFeatureValue(ctx, "links"); err != nil || len(elementsOf(fv.HeldValue())) != 0 {
		t.Errorf("links = %v, %v, want none", fv, err)
	}
}
