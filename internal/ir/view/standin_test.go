package view

import (
	"strings"
	"testing"
)

// standInDOT is the Cameo DOT form of one of the positioned action views of
// standin.sysml, under options.
func standInDOT(t *testing.T, view string, options Options) (*Rendering, string) {
	t.Helper()
	rendering := render(t, "standin.sysml", view)
	options.Style = StyleCameo
	source, err := rendering.DOTWith(options)
	if err != nil {
		t.Fatalf("DOTWith: %v", err)
	}
	checkDOTSyntax(t, source)
	return rendering, source
}

// A join the migration marks as standing for no source node, which no diagram
// positions, is elided from a positioned view: the edge into it and the edge
// out of it are drawn as one along their routes, and a pair without a route is
// left undrawn.
func TestDOTElidesMigrationStandIns(t *testing.T) {
	rendering, source := standInDOT(t, "StandInViews::threadedView", Options{})
	for _, root := range rendering.Roots {
		if found := findStandIn(root.Children); found != nil {
			t.Errorf("stand-in %s survives elision", found.Name)
		}
	}
	for _, node := range allNodes(rendering.Roots) {
		if node.Kind == "metadata" {
			t.Errorf("marker drawn as node %q : %q", node.Name, node.Type)
		}
	}
	if !strings.Contains(source, "// not represented: 1 control node(s) a migration made up, which no diagram positions, elided\n") {
		t.Errorf("elision unaccounted for:\n%s", source)
	}
	if strings.Contains(source, "fillcolor=black") {
		t.Errorf("a bar is drawn for the stand-in:\n%s", source)
	}
	if line := dotLine(source, `"n1" -> "n3"`); !strings.Contains(line, `pos="e,60,100 60,240`) {
		t.Errorf("a to c is not drawn along both routes: %q\n%s", line, source)
	}
	if line := dotLine(source, `"n2" -> "n3"`); !strings.Contains(line, `pos="e,60,100 60,180`) {
		t.Errorf("b to c is not drawn along the route out of the join: %q\n%s", line, source)
	}
}

// A fork the source had but left unnamed carries a synthesized name like a
// stand-in, yet is a node of the diagram: it is kept, and drawn as a bar where
// its routes meet.
func TestDOTKeepsUnnamedControlNodesOfTheSource(t *testing.T) {
	rendering, source := standInDOT(t, "StandInViews::forkedView", Options{})
	var fork *Node
	for _, node := range rendering.Roots[0].Children {
		if node.Kind == "fork" {
			fork = node
		}
	}
	if fork == nil {
		t.Fatalf("the fork is elided:\n%s", source)
	}
	if !fork.NameSynthesized || fork.StandIn {
		t.Errorf("fork: NameSynthesized %v StandIn %v, want true false", fork.NameSynthesized, fork.StandIn)
	}
	if strings.Contains(source, "not represented") {
		t.Errorf("the fork is accounted for as unrepresented:\n%s", source)
	}
	line := dotLine(source, `"n4"`)
	for _, attr := range []string{`fillcolor=black`, `label=""`, `pos="60,180!"`} {
		if !strings.Contains(line, attr) {
			t.Errorf("fork bar lacks %s: %q\n%s", attr, line, source)
		}
	}
	for _, edge := range []string{`"n1" -> "n4" [`, `"n4" -> "n2" [`, `"n4" -> "n3";`} {
		if !strings.Contains(source, "\n  "+edge) {
			t.Errorf("no edge %s through the fork:\n%s", edge, source)
		}
	}
}

// A pin several routed flows leave sits at the mean of where their routes
// start, on the action's border, while each flow keeps its own route.
func TestDOTSharedPinSitsBetweenItsRoutes(t *testing.T) {
	_, source := standInDOT(t, "StandInViews::sharedView", Options{})
	if line := dotLine(source, `"n1.0"`); !strings.Contains(line, `pos="70,134!"`) {
		t.Errorf("shared pin is not between its routes' starts at x=50 and x=90: %q\n%s", line, source)
	}
	for edge, start := range map[string]string{
		`"n1.0" -> "n2.0"`: `pos="e,50,80 50,134`,
		`"n1.0" -> "n3.0"`: `pos="e,270,80 90,134`,
	} {
		if line := dotLine(source, edge); !strings.Contains(line, start) {
			t.Errorf("%s does not keep its own route: %q\n%s", edge, line, source)
		}
	}
}

// An action no diagram positions, drawn in the strip below the drawing, has its
// pins placed on the box the strip gives it.
func TestDOTStripPlacesPinsOfUnplacedActions(t *testing.T) {
	_, source := standInDOT(t, "StandInViews::sharedView", Options{Unplaced: UnplacedStrip})
	action, pin := dotLine(source, `"n4"`), dotLine(source, `"n4.0"`)
	if !strings.Contains(action, `label=<<b>loose : Take</b>>`) || !strings.Contains(action, `pos="51.5,-42!"`) {
		t.Fatalf("loose is not drawn in the strip: %q\n%s", action, source)
	}
	if !strings.Contains(pin, `xlabel="value"`) || !strings.Contains(pin, `pos="51.5,-18!"`) {
		t.Errorf("loose's pin is not on its strip box's top border: %q\n%s", pin, source)
	}
	if dotLine(source, `"n1.0" -> "n4.0"`) == "" {
		t.Errorf("the flow to the strip-drawn pin is undrawn:\n%s", source)
	}
}
