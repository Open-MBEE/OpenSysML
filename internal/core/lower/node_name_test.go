package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A performed action with no name of its own is the node named after the action
// it performs; one declaring a short name declares its name (KerML 7.3.4.5) and
// answers to that short name alone, so a succession naming the performed action
// reaches nothing inside the graph.
func TestShortNamedReferenceAnswersToItsShortNameOnly(t *testing.T) {
	for _, rel := range []string{"::>", "references"} {
		src := "action photo { action d; perform action " + rel + " takePhoto; first d then takePhoto; }"
		graph := actionGraphOf(t, src)
		if !graphHasNode(graph, "takePhoto") || len(graph.Edges) == 0 {
			t.Errorf("%s: an unnamed perform is not the node takePhoto reached by the succession", src)
		}

		src = "action photo { action d; perform action <s> " + rel + " takePhoto; first d then s; }"
		graph = actionGraphOf(t, src)
		var found bool
		for _, node := range graph.Nodes {
			u, ok := node.(*ast.Usage)
			if !ok || u.Ident.ShortName != "s" {
				continue
			}
			found = true
			if name := getNodeName(u); name != "s" {
				t.Errorf("%s: node name = %q, want the short name s", src, name)
			}
			if nodeAnswersTo(u, "takePhoto") {
				t.Errorf("%s: a short-named perform must not answer to takePhoto", src)
			}
		}
		if !found {
			t.Fatalf("%s: short-named usage is not a node of the graph", src)
		}
		if len(graph.Edges) == 0 {
			t.Errorf("%s: succession naming s produced no edge", src)
		}

		src = "action photo { action d; perform action <s> " + rel + " takePhoto; first d then takePhoto; }"
		root := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
		usage := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
		if _, err := ToActionGraph(usage, nil); err == nil || !strings.Contains(err.Error(), `undefined target node "takePhoto"`) {
			t.Errorf("ToActionGraph(%s) error = %v; want takePhoto undefined inside the graph", src, err)
		}
	}
}

func actionGraphOf(t *testing.T, src string) *ActionGraph {
	t.Helper()
	root := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
	usage := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
	graph, err := ToActionGraph(usage, nil)
	if err != nil {
		t.Fatalf("ToActionGraph(%s): %v", src, err)
	}
	return graph
}

func graphHasNode(graph *ActionGraph, name string) bool {
	for _, node := range graph.Nodes {
		if getNodeName(node) == name {
			return true
		}
	}
	return false
}
