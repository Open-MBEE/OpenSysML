package lower

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
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
	if _, err := ToActionGraph(derived, scope); !errors.Is(err, ErrCyclicSpecialization) {
		t.Fatalf("ToActionGraph error = %v, want ErrCyclicSpecialization", err)
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

func TestToActionGraphInheritedActionContentAndMultiplicity(t *testing.T) {
	idx, base, keep, narrow := inheritedStepModel(t)
	resolver := resolve.New(idx)
	graph, err := ToActionGraphWith(keep.Decl, keep.Scope, resolver)
	if err != nil {
		t.Fatalf("lower Keep: %v", err)
	}
	replacement := namedNode(graph, "a")
	if replacement == nil {
		t.Fatal("Keep graph has no action node answering to a")
	}
	var count int
	for _, node := range graph.Nodes {
		if nodeAnswersTo(node, "a") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("Keep graph has %d nodes answering to a, want one", count)
	}
	baseStep, ok := base.Scope.LookupLocal("a")
	if !ok {
		t.Fatal("Base has no action node a")
	}
	var inheritedEdge ast.Node
	for _, edges := range graph.Edges {
		for _, edge := range edges {
			if edge.Target == replacement && edge.Decl != nil && graph.declaredIn[edge.Decl] == base.Scope {
				inheritedEdge = edge.Decl
			}
			if edge.Target == baseStep.Decl {
				t.Fatal("inherited succession still targets Base::a")
			}
		}
	}
	if inheritedEdge == nil {
		t.Fatal("Keep graph has no inherited succession targeting its redefining node")
	}
	baseNode, ok := baseStep.Scope.Node().(*ast.Usage)
	if !ok {
		t.Fatalf("Base::a declaration is %T, want *ast.Usage", baseStep.Scope.Node())
	}
	var inheritedStatement ast.Node
	for _, member := range baseNode.Members {
		if _, ok := unwrapMembership(member).(*ast.AssignmentActionNode); ok {
			inheritedStatement = unwrapMembership(member)
			break
		}
	}
	if inheritedStatement == nil {
		t.Fatal("Base::a has no assignment statement")
	}
	if got := graph.declaredIn[inheritedStatement]; got != baseStep.Scope {
		t.Fatalf("inherited statement scope = %p, want Base::a scope %p", got, baseStep.Scope)
	}

	model := semantics.NewModel(resolver)
	multiplicity, scope := graph.StepMultiplicity(replacement, model)
	if multiplicity == nil || scope != base.Scope {
		t.Fatalf("Keep::a multiplicity = %v in %p, want Base's declaration in %p", multiplicity, scope, base.Scope)
	}
	if got, err := graph.StepCount(replacement, model); err != nil || got != 3 {
		t.Fatalf("Keep::a StepCount = %d, %v; want 3", got, err)
	}

	narrowGraph, err := ToActionGraphWith(narrow.Decl, narrow.Scope, resolver)
	if err != nil {
		t.Fatalf("lower Narrow: %v", err)
	}
	narrowStep := namedNode(narrowGraph, "a")
	multiplicity, scope = narrowGraph.StepMultiplicity(narrowStep, model)
	if multiplicity == nil || scope != narrowGraph.Scopes[narrowStep] {
		t.Fatalf("Narrow::a multiplicity = %v in %p, want its own node scope %p", multiplicity, scope, narrowGraph.Scopes[narrowStep])
	}
	if got, err := narrowGraph.StepCount(narrowStep, model); err != nil || got != 2 {
		t.Fatalf("Narrow::a StepCount = %d, %v; want 2", got, err)
	}
}

func TestToActionGraphInheritsPerformStatements(t *testing.T) {
	src := `
		action def Foo;
		action def Base { perform action p : Foo; }
		action def Derived :> Base;
	`
	derived, scope, root := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("lower Derived: %v", err)
	}
	base := actionDefinition(t, root, "Base")
	baseScope := scope.Parent().ChildFor(base)
	for _, node := range graph.Nodes {
		usage, ok := node.(*ast.Usage)
		if !ok || !IsPerformedActionUsage(usage) {
			continue
		}
		if got := graph.declaredIn[node]; got != baseScope {
			t.Fatalf("inherited perform scope = %p, want Base scope %p", got, baseScope)
		}
		return
	}
	t.Fatal("Derived graph has no inherited perform node")
}

func TestToActionGraphInheritedFlowStartsAlongsideUnorderedLocalNode(t *testing.T) {
	src := `
		action def B1 {
			attribute c : Integer = 0;
			first start then a;
			action a { assign c := c + 1; }
			then done;
		}
		action def P0 :> B1;
		action def P3 :> B1 {
			action b { assign c := c + 100; }
		}
	`
	decl, scope, _ := inheritedActionDecl(t, src, "P3")
	graph, err := ToActionGraph(decl, scope)
	if err != nil {
		t.Fatalf("lower P3: %v", err)
	}
	StartFlow(graph)
	if graph.Initial == nil || getNodeName(graph.Initial) != "start" {
		t.Fatalf("P3 initial = %v, want the inherited start marker", graph.Initial)
	}
	a, b := namedNode(graph, "a"), namedNode(graph, "b")
	if a == nil || b == nil {
		t.Fatalf("P3 nodes: inherited a = %v, local b = %v", a, b)
	}
	if edges := graph.Edges[graph.Initial]; len(edges) != 1 || edges[0].Target != a {
		t.Fatalf("inherited start edges = %v, want one edge to a", edges)
	}
	if len(graph.Concurrent) != 1 || graph.Concurrent[0] != b {
		t.Fatalf("concurrent starts = %d, want only local b", len(graph.Concurrent))
	}
}

func TestToActionGraphMergesBodyStatingTypedActionNode(t *testing.T) {
	src := `
		action def B1 {
			attribute c : Integer = 0;
			first start then a;
			action a { assign c := c + 1; }
			then done;
		}
		action def Host {
			action y : B1 { action b { assign c := c + 100; } }
		}
	`
	host, scope, _ := inheritedActionDecl(t, src, "Host")
	graph, err := ToActionGraph(host, scope)
	if err != nil {
		t.Fatalf("lower Host: %v", err)
	}
	y, ok := namedNode(graph, "y").(*ast.Usage)
	if !ok {
		t.Fatalf("y node = %T, want *ast.Usage", namedNode(graph, "y"))
	}
	subflow := graph.Subflows[y]
	if subflow == nil || subflow.Graph == nil {
		t.Fatalf("y subflow = %#v, want a merged typed graph", subflow)
	}
	if _, ok := graph.MergedTypedSubflows[y]; !ok {
		t.Fatal("typed subflow was not marked as already merged")
	}
	a, b := namedNode(subflow.Graph, "a"), namedNode(subflow.Graph, "b")
	if a == nil || b == nil {
		t.Fatalf("typed subflow nodes: inherited a = %v, local b = %v", a, b)
	}
	if edges := subflow.Graph.Edges[subflow.Graph.Initial]; len(edges) != 1 || edges[0].Target != a {
		t.Fatalf("typed start edges = %v, want one edge to inherited a", edges)
	}
	if len(subflow.Graph.Concurrent) != 1 || subflow.Graph.Concurrent[0] != b {
		t.Fatalf("typed concurrent starts = %v, want local b", subflow.Graph.Concurrent)
	}
}

func TestToActionGraphBodyStatingTypedNodeIncludesTypedPins(t *testing.T) {
	src := `
		action def Scale {
			in x : Integer;
			in factor : Integer = 10;
			out y : Integer;
			first start then scaling;
			action scaling { assign y := x * factor; }
			then done;
		}
		action def Host {
			action scaled : Scale {
				in x = 3;
				assign y := y + 1;
			}
		}
	`
	host, scope, _ := inheritedActionDecl(t, src, "Host")
	graph, err := ToActionGraph(host, scope)
	if err != nil {
		t.Fatalf("lower Host: %v", err)
	}
	node, ok := namedNode(graph, "scaled").(*ast.Usage)
	if !ok {
		t.Fatalf("scaled node = %T, want *ast.Usage", namedNode(graph, "scaled"))
	}
	features := make(map[string]Feature, len(graph.Features[node]))
	for _, feature := range graph.Features[node] {
		features[feature.Name] = feature
	}
	for name, direction := range map[string]ast.FeatureDirection{
		"x":      ast.DirIn,
		"factor": ast.DirIn,
		"y":      ast.DirOut,
	} {
		feature, ok := features[name]
		if !ok || feature.Direction != direction {
			t.Errorf("typed feature %q = %#v, present %v; want direction %v", name, feature, ok, direction)
		}
	}
	if features["factor"].Value == nil {
		t.Error("typed default for factor was not lowered")
	}
}

func TestToActionGraphPinOnlyTypedUsageKeepsInvocation(t *testing.T) {
	src := `
		metadata def Marker;
		action def Base {
			first start then a;
			action a;
			then done;
		}
		action usage : Base {
			metadata marker : Marker;
			in value = 3;
		}
	`
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	var usage *ast.Usage
	for _, member := range root.Members {
		candidate, ok := unwrapMembership(member).(*ast.Usage)
		if ok && candidate.Kind == ast.UsageAction && getNodeName(candidate) == "usage" {
			usage = candidate
			break
		}
	}
	if usage == nil {
		t.Fatal("action usage not found")
	}
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	scope := idx.DocumentRoot("test.sysml").ChildFor(usage)
	if scope == nil {
		t.Fatal("scope for action usage missing")
	}
	graph, err := ToActionGraphWith(usage, scope, resolve.New(idx))
	if err != nil {
		t.Fatalf("lower action usage: %v", err)
	}
	if inherited := namedNode(graph, "a"); inherited != nil {
		t.Fatalf("pin-only typed usage adopted type body node %v", inherited)
	}
	if _, merged := graph.MergedTypedSubflows[usage]; merged {
		t.Fatal("pin-only typed usage with metadata was marked as a merged subflow")
	}
}

func TestToActionGraphFeatureOnlySelfTypedNodeKeepsInvocation(t *testing.T) {
	src := `
		action def A {
			attribute c : Integer := 0;
			action x : A[0..*] {
				ref occurrence r :>> self;
			}
		}
	`
	decl, scope, _ := inheritedActionDecl(t, src, "A")
	graph, err := ToActionGraph(decl, scope)
	if err != nil {
		t.Fatalf("lower A: %v", err)
	}
	node, ok := namedNode(graph, "x").(*ast.Usage)
	if !ok {
		t.Fatalf("x node = %T, want *ast.Usage", namedNode(graph, "x"))
	}
	if subflow := graph.Subflows[node]; subflow != nil {
		t.Fatalf("x subflow = %#v, want no merged typed subflow", subflow)
	}
	if _, merged := graph.MergedTypedSubflows[node]; merged {
		t.Fatal("feature-only self-typed x was marked as a merged typed subflow")
	}
}

func TestToActionGraphExecutableSelfTypedNodeRefusesRecursiveMerge(t *testing.T) {
	src := `
		action def A {
			attribute c : Integer := 0;
			first start then x;
			action x : A {
				assign c := c + 1;
			}
			then done;
		}
	`
	decl, scope, _ := inheritedActionDecl(t, src, "A")
	graph, err := ToActionGraph(decl, scope)
	if err != nil {
		t.Fatalf("lower A: %v", err)
	}
	node := namedNode(graph, "x")
	subflow := graph.Subflows[node]
	if subflow == nil || !errors.Is(subflow.Err, ErrRecursiveActionTyping) {
		t.Fatalf("x subflow = %#v, want ErrRecursiveActionTyping", subflow)
	}
}

func TestToActionGraphBodyStatingTypedNodeMergesFiniteGeneral(t *testing.T) {
	src := `
		action def Base {
			attribute c : Integer := 0;
		}
		action def P :> Base {
			attribute d : Integer := 0;
			first start then x;
			action x : Base {
				assign c := c + 1;
			}
			then done;
		}
	`
	decl, scope, _ := inheritedActionDecl(t, src, "P")
	graph, err := ToActionGraph(decl, scope)
	if err != nil {
		t.Fatalf("lower P: %v", err)
	}
	node := namedNode(graph, "x")
	subflow := graph.Subflows[node]
	if subflow == nil || subflow.Graph == nil || subflow.Err != nil {
		t.Fatalf("x subflow = %#v, want a successfully merged typed subflow", subflow)
	}
	if _, merged := graph.MergedTypedSubflows[node]; !merged {
		t.Fatal("x was not marked as a merged typed subflow")
	}
}

func TestToActionGraphInheritedTypedBodyRecursionIsRefused(t *testing.T) {
	src := `
		action def Base {
			attribute c : Integer := 0;
			action x : P {
				assign c := c + 1;
			}
		}
		action def P :> Base;
	`
	decl, scope, _ := inheritedActionDecl(t, src, "Base")
	graph, err := ToActionGraph(decl, scope)
	if err != nil {
		t.Fatalf("lower Base: %v", err)
	}
	outer := namedNode(graph, "x")
	outerSubflow := graph.Subflows[outer]
	if outerSubflow == nil || outerSubflow.Graph == nil || outerSubflow.Err != nil {
		t.Fatalf("outer x subflow = %#v, want P's graph", outerSubflow)
	}
	inner := namedNode(outerSubflow.Graph, "x")
	innerSubflow := outerSubflow.Graph.Subflows[inner]
	if innerSubflow == nil || !errors.Is(innerSubflow.Err, ErrRecursiveActionTyping) {
		t.Fatalf("inner x subflow = %#v, want ErrRecursiveActionTyping", innerSubflow)
	}
}

func TestToActionGraphExecutableMutuallyTypedNodesRefuseRecursiveMerge(t *testing.T) {
	src := `
		action def A {
			attribute c : Integer := 0;
			action y : B {
				assign c := c + 1;
			}
		}
		action def B {
			attribute c : Integer := 0;
			action z : A {
				assign c := c + 1;
			}
		}
	`
	decl, scope, _ := inheritedActionDecl(t, src, "A")
	graph, err := ToActionGraph(decl, scope)
	if err != nil {
		t.Fatalf("lower A: %v", err)
	}
	y := namedNode(graph, "y")
	subflow := graph.Subflows[y]
	if subflow == nil || subflow.Graph == nil {
		t.Fatalf("y subflow = %#v, want B's graph", subflow)
	}
	z := namedNode(subflow.Graph, "z")
	recursive := subflow.Graph.Subflows[z]
	if recursive == nil || !errors.Is(recursive.Err, ErrRecursiveActionTyping) {
		t.Fatalf("z subflow = %#v, want ErrRecursiveActionTyping", recursive)
	}
}

func TestToActionGraphTypedUsageOverridesMatchingInheritedSuccession(t *testing.T) {
	src := `package test {
		action def Base {
			action a;
			first start then a;
		}
		action use : Base {
			first start then a;
		}
	}`
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	usageSymbols := idx.LookupQualified("test::use")
	if len(usageSymbols) != 1 {
		t.Fatalf("test::use matched %d symbols, want one", len(usageSymbols))
	}
	usage, ok := usageSymbols[0].Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("test::use declaration is %T, want *ast.Usage", usageSymbols[0].Decl)
	}
	scope := usageSymbols[0].Scope
	graph, err := ToActionGraphWith(usage, scope, resolve.New(idx))
	if err != nil {
		t.Fatalf("lower typed usage: %v", err)
	}
	var edges int
	for _, outgoing := range graph.Edges {
		edges += len(outgoing)
	}
	if edges != 1 {
		t.Fatalf("typed usage graph has %d edges, want one own edge overriding the inherited succession", edges)
	}
}

func TestToActionGraphInheritedActionNodeFeaturesKeepDeclarationScope(t *testing.T) {
	src := `package test {
		action def Foo {
			in ref context : Base[1];
			out c : Integer = 0;
		}
		action def Base {
			action a : Foo {
				accept payload : Integer;
				in ref :>> context = this;
			}
		}
		action def Derived :> Base { action :>> a[2]; }
	}`
	idx := libs.NewModelIndex()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocument("test.sysml", root)
	idx.ExpandWildcardImports()
	derived := idx.LookupQualified("test::Derived")
	if len(derived) != 1 {
		t.Fatalf("Derived matched %d symbols, want one", len(derived))
	}
	resolver := resolve.New(idx)
	graph, err := ToActionGraphWith(derived[0].Decl, derived[0].Scope, resolver)
	if err != nil {
		t.Fatalf("lower Derived: %v", err)
	}
	node := namedNode(graph, "a")
	if node == nil {
		t.Fatal("Derived graph has no action node answering to a")
	}
	base := idx.LookupQualified("test::Base")
	if len(base) != 1 {
		t.Fatalf("Base matched %d symbols, want one", len(base))
	}
	baseStep, ok := base[0].Scope.LookupLocal("a")
	if !ok {
		t.Fatal("Base::a not found")
	}
	performed, ok := graph.Performs[node]
	if !ok || performed.Target == nil || performed.Target.Text() != "Foo" || performed.Scope != base[0].Scope {
		t.Fatalf("Derived::a performed type = %+v, want Foo from Base's scope", performed)
	}
	subflow := graph.Subflows[node]
	if subflow == nil || subflow.Graph == nil {
		t.Fatal("Derived::a has no inherited accept subflow")
	}
	var acceptNode ast.Node
	for _, candidate := range subflow.Graph.Nodes {
		usage, ok := candidate.(*ast.Usage)
		if ok && acceptsMessage(usage) {
			acceptNode = candidate
			break
		}
	}
	accept, ok := subflow.Graph.Accepts[acceptNode]
	if acceptNode == nil || !ok || accept.ParamName != "payload" ||
		accept.Scope != subflow.Graph.Scopes[acceptNode] {
		t.Fatalf("Derived::a inherited accept = %+v on %v, want payload in the inherited accept node's scope",
			accept, acceptNode)
	}
	var inheritedPin *ast.Usage
	for _, feature := range graph.Features[node] {
		if feature.Name == "context" {
			inheritedPin, _ = feature.Node.(*ast.Usage)
			if feature.Scope == nil || feature.Scope.Node() == nil {
				t.Fatalf("inherited context pin scope = %v, want its declaration scope", feature.Scope)
			}
			break
		}
	}
	if inheritedPin == nil {
		t.Fatal("Derived::a has no inherited context pin")
	}
	if got := graph.declaredIn[inheritedPin]; got != baseStep.Scope {
		t.Fatalf("inherited context pin declaring scope = %v, want Base::a's scope", got)
	}
}

func TestToActionGraphInheritsTypingAcrossTwoRedefinitions(t *testing.T) {
	src := `
		action def Foo {
			out c : Integer = 0;
			assign c := c + 1;
		}
		action def Base { action a : Foo; }
		action def Mid :> Base { action :>> a; }
		action def Narrow :> Mid { action :>> a[2]; }
	`
	narrow, scope, root := inheritedActionDecl(t, src, "Narrow")
	graph, err := ToActionGraph(narrow, scope)
	if err != nil {
		t.Fatalf("lower Narrow: %v", err)
	}
	node := namedNode(graph, "a")
	if node == nil {
		t.Fatal("Narrow graph has no action node answering to a")
	}
	performed, ok := graph.Performs[node]
	baseScope := scope.Parent().ChildFor(actionDefinition(t, root, "Base"))
	if !ok || performed.Target == nil || performed.Target.Text() != "Foo" || performed.Scope != baseScope {
		t.Fatalf("Narrow::a performed type = %s in %p, want Foo from Base's scope %p",
			ast.SimpleName(performed.Target), performed.Scope, baseScope)
	}
}

func inheritedStepModel(t *testing.T) (*symbols.Index, *symbols.Symbol, *symbols.Symbol, *symbols.Symbol) {
	t.Helper()
	src := `package test {
		private import ScalarValues::*;
		action def Base {
			attribute c : Integer = 0;
			first start then a;
			action a[3] { assign c := c + 1; }
			then done;
		}
		action def Keep :> Base {
			action :>> a { assign c := c + 10; }
		}
		action def Narrow :> Base {
			action :>> a[2];
		}
	}`
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("test.sysml", root)
	idx.ExpandWildcardImports()
	lookup := func(name string) *symbols.Symbol {
		t.Helper()
		matches := idx.LookupQualified("test::" + name)
		if len(matches) != 1 {
			t.Fatalf("test::%s matched %d symbols, want one", name, len(matches))
		}
		return matches[0]
	}
	return idx, lookup("Base"), lookup("Keep"), lookup("Narrow")
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
	add, fin, extra, idle := namedNode(graph, "add"), namedNode(graph, "fin"), namedNode(graph, "extra"), namedNode(graph, "idle")
	if add == nil || fin == nil || extra == nil || idle == nil {
		t.Fatal("inherited action nodes were not collected")
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
		{idle, "k", "x", baseBody},
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
	if len(flows) != 2 {
		t.Fatalf("data flows out of add = %+v, want inherited flows to fin and idle", flows)
	}
	targets := map[ast.Node]bool{}
	for _, flow := range flows {
		targets[flow.Target] = true
	}
	if !targets[fin] || !targets[idle] {
		t.Fatalf("data flows out of add target %v, want fin and idle", targets)
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
