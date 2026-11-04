package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// usageStateGraph lowers the state usage named in package M of src through
// the name-resolution tier, the way the runtime does.
func usageStateGraph(t *testing.T, src, name string) *StateGraph {
	t.Helper()
	p := parser.New(source.New("m.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("m.sysml", root)
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot("m.sysml").LookupLocal("M")
	if !ok {
		t.Fatal("package M not indexed")
	}
	u, ok := pkg.Scope.LookupLocal(name)
	if !ok {
		t.Fatalf("state %s not indexed", name)
	}
	graph, err := ToStateGraphWithEndpoints(u.Decl, u.Scope, NewLibraryStateTypes(resolve.New(idx)))
	if err != nil {
		t.Fatalf("ToStateGraph(%s): %v", name, err)
	}
	return graph
}

const stateUsageParametersSource = `package M {
	state def Def {
		in n;
		attribute hidden;
		entry; then s1;
		state s1;
		state s2;
		transition first s1 if n > 2 then s2;
	}
	state valued : Def {
		in n = 3;
		attribute hidden = 1;
	}
	state diamond : Def :> valued;
}
`

// A usage's same-named parameter redefines the definition's by position
// (KerML 1.1 §8.4.4.7.2), taking its place among the machine's attributes, while
// a same-named attribute that is no parameter only hides it.
func TestToStateGraphUsageParameterRedefinesByPosition(t *testing.T) {
	graph := usageStateGraph(t, stateUsageParametersSource, "valued")
	var names []string
	for _, attr := range graph.Attributes {
		names = append(names, attr.Name)
		if attr.Name == "n" && (attr.Value == nil || attr.Shadowed) {
			t.Errorf("n = %+v, want the usage's valued parameter alone", attr)
		}
	}
	if len(names) != 3 {
		t.Fatalf("attributes = %v, want n and both hidden", names)
	}
}

// A definition reached along two paths — the usage's typing and the valued usage
// it subsets — contributes its content once.
func TestToStateGraphDiamondContributesOnce(t *testing.T) {
	graph := usageStateGraph(t, stateUsageParametersSource, "diamond")
	if len(graph.States) != 2 {
		t.Fatalf("states = %d, want s1 and s2 once", len(graph.States))
	}
	var n int
	for _, attr := range graph.Attributes {
		if attr.Name == "n" {
			n++
			if attr.Value == nil {
				t.Errorf("n = %+v, want the subsetted usage's value", attr)
			}
		}
	}
	if n != 1 {
		t.Errorf("n held %d times, want once", n)
	}
}

// A usage typed by a definition that connects its ports carries the definition's
// connections, which a send from the inherited body routes over.
func TestToStateGraphUsageInheritsConnections(t *testing.T) {
	graph := usageStateGraph(t, `package M {
	port def P;
	state def Def {
		port a : P;
		port b : P;
		connect a to b;
		entry; then s1;
		state s1;
	}
	state usage : Def;
}
`, "usage")
	if len(graph.Connections) != 1 {
		t.Fatalf("connections = %d, want the definition's one", len(graph.Connections))
	}
}
