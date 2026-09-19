package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestToActionGraphInheritedActionNode(t *testing.T) {
	src := `
		action def Base { action a; }
		action def Derived :> Base { first start then a; }
	`
	derived, scope, _ := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	inherited := namedNode(graph, "a")
	if inherited == nil {
		t.Fatal("inherited action node was not collected")
	}
	if edges := graph.Edges[graph.Initial]; len(edges) != 1 || edges[0].Target != inherited {
		t.Fatalf("initial edges = %v, want one edge to inherited a", edges)
	}
}

func TestToActionGraphInheritedActionNodeThroughTwoSpecializations(t *testing.T) {
	src := `
		action def Grand { action a; }
		action def Base :> Grand;
		action def Derived :> Base { first start then a; }
	`
	derived, scope, _ := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	if namedNode(graph, "a") == nil {
		t.Fatal("two-level inherited action node was not collected")
	}
}

func TestToActionGraphQualifiedInheritedActionNode(t *testing.T) {
	src := `
		action def Base { action a; }
		action def Derived :> Base { first start then Base::a; }
	`
	derived, scope, _ := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	inherited := namedNode(graph, "a")
	if inherited == nil {
		t.Fatal("qualified inherited action node was not collected")
	}
	if edges := graph.Edges[graph.Initial]; len(edges) != 1 || edges[0].Target != inherited {
		t.Fatalf("initial edges = %v, want one edge to qualified inherited a", edges)
	}
}

func TestToActionGraphLocalActionShadowsInheritedNode(t *testing.T) {
	src := `
		action def Base { action a; }
		action def Derived :> Base { action a; first start then a; }
	`
	derived, scope, root := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	local := actionMember(t, root, "Derived", "a")
	if namedNode(graph, "a") != local {
		t.Fatalf("a node = %p, want local declaration %p", namedNode(graph, "a"), local)
	}
}

func TestToActionGraphDoesNotAdoptInheritedInitialOrFinal(t *testing.T) {
	src := `
		action def Base {
			first start;
			action a;
			done;
			succession first start then a;
			succession first a then done;
		}
		action def Derived :> Base { first start then a; }
	`
	derived, scope, root := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	base := actionDefinition(t, root, "Base")
	baseScope := scope.Parent().ChildFor(base)
	for _, node := range base.Members {
		switch n := unwrapMembership(node).(type) {
		case *ast.InitialNode, *ast.FinalNode:
			for _, graphNode := range graph.Nodes {
				if graphNode == n {
					t.Fatalf("inherited %T became a node of the derived graph", n)
				}
			}
			if baseScope == nil {
				t.Fatal("base body scope missing")
			}
		}
	}
	if _, ok := graph.Initial.(*ast.InitialNode); !ok {
		t.Fatalf("derived initial = %T, want synthesized initial", graph.Initial)
	}
}

func TestToActionGraphInheritedActionNodeCycleTerminates(t *testing.T) {
	src := `
		action def A :> B;
		action def B :> A;
		action def Derived :> A { first start then missing; }
	`
	derived, scope, _ := inheritedActionDecl(t, src, "Derived")
	if _, err := ToActionGraph(derived, scope); err == nil {
		t.Fatal("expected an undefined endpoint error")
	}
}

func inheritedActionDecl(t *testing.T, src, name string) (ast.Node, *symbols.Scope, *ast.RootNamespace) {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	doc := idx.DocumentRoot("test.sysml")
	decl := actionDefinition(t, root, name)
	scope := doc.ChildFor(decl)
	if scope == nil {
		t.Fatalf("scope for %s missing", name)
	}
	return decl, scope, root
}

func actionDefinition(t *testing.T, root *ast.RootNamespace, name string) *ast.Definition {
	t.Helper()
	var find func([]ast.Node) *ast.Definition
	find = func(members []ast.Node) *ast.Definition {
		for _, member := range members {
			switch n := unwrapMembership(member).(type) {
			case *ast.Definition:
				if n.Kind == ast.DefAction && n.Ident.Name == name {
					return n
				}
				if found := find(n.Members); found != nil {
					return found
				}
			case *ast.Package:
				if found := find(n.Members); found != nil {
					return found
				}
			}
		}
		return nil
	}
	if decl := find(root.Members); decl != nil {
		return decl
	}
	t.Fatalf("action definition %s not found", name)
	return nil
}

func actionMember(t *testing.T, root *ast.RootNamespace, action, member string) ast.Node {
	t.Helper()
	decl := actionDefinition(t, root, action)
	for _, candidate := range decl.Members {
		if n := unwrapMembership(candidate); getNodeName(n) == member {
			return n
		}
	}
	t.Fatalf("action member %s not found", member)
	return nil
}

func namedNode(graph *ActionGraph, name string) ast.Node {
	for _, node := range graph.Nodes {
		if nodeAnswersTo(node, name) {
			return node
		}
	}
	return nil
}

// A binding or flow a base action states at a pin of a node the derived action
// inherits reaches the derived graph, in the base's scope and once even when the
// base is reached along two generalization paths; one at a node the derived
// action does not sequence lowers to nothing.
func TestToActionGraphInheritedPinConnections(t *testing.T) {
	src := `
		action def Base {
			attribute x : Integer = 5;
			action add { in a : Integer; out sum : Integer; }
			action fin { in n : Integer; }
			action idle { in k : Integer; }
			bind add.a = x;
			bind idle.k = x;
			flow add.sum to fin.n;
			flow add.sum to idle.k;
		}
		action def Mid :> Base {
			attribute y : Integer = 1;
			action extra { in m : Integer; }
			bind extra.m = y;
		}
		action def Derived :> Mid, Base {
			bind extra.m = add.sum;
			first start then add;
			succession add then fin;
			succession fin then extra;
			succession extra then done;
		}
	`
	derived, scope, root := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	add, fin, extra := namedNode(graph, "add"), namedNode(graph, "fin"), namedNode(graph, "extra")
	if add == nil || fin == nil || extra == nil {
		t.Fatal("inherited action nodes were not collected")
	}
	if namedNode(graph, "idle") != nil {
		t.Fatal("a node the derived action does not sequence became a node of its graph")
	}

	baseBody := scope.Parent().ChildFor(actionDefinition(t, root, "Base"))
	midBody := scope.Parent().ChildFor(actionDefinition(t, root, "Mid"))
	want := []struct {
		node  ast.Node
		pin   string
		other string
		scope *symbols.Scope
	}{
		{extra, "m", "add.sum", scope},
		{add, "sum", "extra.m", scope},
		{extra, "m", "y", midBody},
		{add, "a", "x", baseBody},
	}
	if len(graph.Bindings) != len(want) {
		t.Fatalf("lowered %d pin bindings, want %d: %+v", len(graph.Bindings), len(want), graph.Bindings)
	}
	for i, w := range want {
		got := graph.Bindings[i]
		if got.Node != w.node || got.Pin != w.pin || FeaturePath(got.Other) != w.other {
			t.Errorf("binding %d = %s.%s = %s, want %s.%s = %s",
				i, getNodeName(got.Node), got.Pin, FeaturePath(got.Other), getNodeName(w.node), w.pin, w.other)
		}
		if w.scope == nil || got.Scope != w.scope {
			t.Errorf("binding %d is scoped to %s, want its declaring action's body", i, scopeName(got.Scope))
		}
	}

	flows := graph.DataFlows[add]
	if len(flows) != 1 || flows[0].Target != fin || flows[0].SourcePin != "sum" || flows[0].TargetPin != "n" {
		t.Fatalf("data flows out of add = %+v, want one flow add.sum to fin.n", flows)
	}
}

// A base's connector follows its node's declaration: a same-named local node does
// not take it (it lowers to nothing), a node redefining the base's does.
func TestToActionGraphInheritedPinConnectionsFollowDeclarationIdentity(t *testing.T) {
	src := `
		action def Base {
			attribute x : Integer = 5;
			action src { out n : Integer; }
			action add { in a : Integer; out sum : Integer; }
			action fin { in n : Integer; }
			bind add.a = x;
			bind fin.n = src.n;
			flow add.sum to fin.n;
		}
		action def Masked :> Base {
			action add { out total : Integer; }
			action fin { in k : Integer; }
			first start then add;
			succession add then fin;
			then done;
		}
		action def HalfReplaced :> Base {
			action src { out n : Integer; }
			first start then src;
			succession src then add;
			succession add then fin;
			then done;
		}
		action def Redefined :> Base {
			action add :>> add { in a : Integer; out sum : Integer; }
			first start then src;
			succession src then add;
			succession add then fin;
			then done;
		}
		action def Twice :> Redefined {
			action again :>> Redefined::add { in a : Integer; out sum : Integer; }
			first start then src;
			succession src then again;
			succession again then fin;
			then done;
		}
	`
	masked, scope, root := inheritedActionDecl(t, src, "Masked")
	graph, err := ToActionGraph(masked, scope)
	if err != nil {
		t.Fatalf("ToActionGraph(Masked): %v", err)
	}
	if namedNode(graph, "add") != actionMember(t, root, "Masked", "add") {
		t.Fatal("the local add is not the graph's add")
	}
	if len(graph.Bindings) != 0 {
		t.Errorf("a base binding was lowered at a local node taking its node's name: %+v", graph.Bindings)
	}
	for source, flows := range graph.DataFlows {
		t.Errorf("a base flow out of %s was lowered at local nodes taking its nodes' names: %+v", getNodeName(source), flows)
	}

	half, scope, root := inheritedActionDecl(t, src, "HalfReplaced")
	graph, err = ToActionGraph(half, scope)
	if err != nil {
		t.Fatalf("ToActionGraph(HalfReplaced): %v", err)
	}
	inheritedAdd, inheritedFin := actionMember(t, root, "Base", "add"), actionMember(t, root, "Base", "fin")
	if namedNode(graph, "src") != actionMember(t, root, "HalfReplaced", "src") || namedNode(graph, "fin") != inheritedFin {
		t.Fatal("HalfReplaced: src is not the local node or fin is not the inherited declaration")
	}
	if len(graph.Bindings) != 1 || graph.Bindings[0].Node != inheritedAdd || graph.Bindings[0].Pin != "a" {
		t.Errorf("HalfReplaced: bindings = %+v, want only x at add.a; a binding at src.n, whose src the action replaced, must not reach fin.n", graph.Bindings)
	}
	if flows := graph.DataFlows[inheritedAdd]; len(flows) != 1 || flows[0].Target != inheritedFin {
		t.Errorf("HalfReplaced: data flows out of add = %+v, want one flow to the inherited fin", flows)
	}

	for _, tc := range []struct{ action, node string }{{"Redefined", "add"}, {"Twice", "again"}} {
		derived, scope, root := inheritedActionDecl(t, src, tc.action)
		graph, err := ToActionGraph(derived, scope)
		if err != nil {
			t.Fatalf("ToActionGraph(%s): %v", tc.action, err)
		}
		redefining := actionMember(t, root, tc.action, tc.node)
		fin := namedNode(graph, "fin")
		if fin == nil || fin != actionMember(t, root, "Base", "fin") {
			t.Fatalf("%s: fin = %v, want the inherited declaration", tc.action, fin)
		}
		baseBody := scope.Parent().ChildFor(actionDefinition(t, root, "Base"))
		src := actionMember(t, root, "Base", "src")
		if len(graph.Bindings) != 3 || graph.Bindings[0].Node != redefining || graph.Bindings[0].Pin != "a" ||
			FeaturePath(graph.Bindings[0].Other) != "x" || graph.Bindings[0].Scope != baseBody ||
			graph.Bindings[1].Node != fin || graph.Bindings[1].OtherNode != src ||
			graph.Bindings[2].Node != src || graph.Bindings[2].OtherNode != fin {
			t.Errorf("%s: bindings = %+v, want x at %s.a in Base's scope, then fin.n = src.n at both inherited ends", tc.action, graph.Bindings, tc.node)
		}
		flows := graph.DataFlows[redefining]
		if len(flows) != 1 || flows[0].Target != fin || flows[0].SourcePin != "sum" || flows[0].TargetPin != "n" {
			t.Errorf("%s: data flows out of %s = %+v, want one flow to fin.n", tc.action, tc.node, flows)
		}
	}
}

func scopeName(s *symbols.Scope) string {
	if s == nil {
		return "<nil>"
	}
	if def, ok := s.Node().(*ast.Definition); ok {
		return def.Ident.Name
	}
	return getNodeName(s.Node())
}
