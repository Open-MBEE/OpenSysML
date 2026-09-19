package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestScopeDefineAndLookupLocal(t *testing.T) {
	root := NewScope(nil, nil)
	sym := &Symbol{Name: "P", Kind: SymbolPackage}
	root.Define("P", sym)

	got, ok := root.LookupLocal("P")
	if !ok || got != sym {
		t.Fatalf("LookupLocal(P) = %v, %v; want the defined symbol", got, ok)
	}
	if _, ok := root.LookupLocal("Q"); ok {
		t.Fatalf("LookupLocal(Q) unexpectedly found a symbol")
	}
}

func TestScopeShortAndPrimaryName(t *testing.T) {
	root := NewScope(nil, nil)
	sym := &Symbol{Name: "Vehicle", Kind: SymbolPackage}
	root.Define("v", sym)       // short name
	root.Define("Vehicle", sym) // primary name

	for _, key := range []string{"v", "Vehicle"} {
		got, ok := root.LookupLocal(key)
		if !ok || got != sym {
			t.Fatalf("LookupLocal(%q) = %v, %v; want the symbol", key, got, ok)
		}
	}
}

func TestScopeParentAndChildren(t *testing.T) {
	root := NewScope(nil, nil)
	pkgNode := &ast.Package{}
	child := NewScope(root, pkgNode)
	root.AddChild(child)

	if child.Parent() != root {
		t.Fatalf("child.Parent() != root")
	}
	if len(root.Children()) != 1 || root.Children()[0] != child {
		t.Fatalf("root.Children() = %v; want [child]", root.Children())
	}
	if child.Node() != pkgNode {
		t.Fatalf("child.Node() != pkgNode")
	}
}

func TestScopeChildForFindsTheScopeADeclarationOwns(t *testing.T) {
	root := NewScope(nil, nil)
	first, second := &ast.Package{}, &ast.Package{}
	firstScope, secondScope := NewScope(root, first), NewScope(root, second)
	root.AddChild(firstScope)
	root.AddChild(secondScope)
	root.AddChild(NewScope(root, nil)) // a scope no declaration owns

	if got := root.ChildFor(first); got != firstScope {
		t.Errorf("ChildFor(first) = %v, want the first child", got)
	}
	if got := root.ChildFor(second); got != secondScope {
		t.Errorf("ChildFor(second) = %v, want the second child", got)
	}
	if got := root.ChildFor(&ast.Package{}); got != nil {
		t.Errorf("ChildFor(a declaration with no scope here) = %v, want nil", got)
	}
	if got := root.ChildFor(nil); got != nil {
		t.Errorf("ChildFor(nil) = %v, want nil", got)
	}
}

// A scope with more children than the scan threshold answers from its index,
// including after a child is added once that index has been built.
func TestScopeChildForAboveIndexThreshold(t *testing.T) {
	root := NewScope(nil, nil)
	nodes := make([]ast.Node, childIndexThreshold+2)
	scopes := make([]*Scope, len(nodes))
	for i := range nodes {
		nodes[i] = &ast.Package{}
		scopes[i] = NewScope(root, nodes[i])
		root.AddChild(scopes[i])
		root.AddChild(NewScope(root, nil)) // a scope no declaration owns
	}
	for i, node := range nodes {
		if got := root.ChildFor(node); got != scopes[i] {
			t.Fatalf("ChildFor(child %d) = %v, want that child's scope", i, got)
		}
	}
	if got := root.ChildFor(&ast.Package{}); got != nil {
		t.Errorf("ChildFor(a declaration with no scope here) = %v, want nil", got)
	}
	added := &ast.Package{}
	addedScope := NewScope(root, added)
	root.AddChild(addedScope)
	if got := root.ChildFor(added); got != addedScope {
		t.Errorf("ChildFor(a child added after indexing) = %v, want that child", got)
	}
}

// Two children of one node are one body scoped twice, so the first wins.
func TestScopeChildForPrefersTheFirstChildOfANode(t *testing.T) {
	for _, children := range []int{2, childIndexThreshold + 2} {
		root := NewScope(nil, nil)
		node := &ast.Package{}
		first := NewScope(root, node)
		root.AddChild(first)
		root.AddChild(NewScope(root, node))
		for len(root.Children()) < children {
			root.AddChild(NewScope(root, &ast.Package{}))
		}
		if got := root.ChildFor(node); got != first {
			t.Errorf("ChildFor with %d children = %v, want the first child", children, got)
		}
	}
}

func TestScopeDefineDuplicateKeepsAll(t *testing.T) {
	root := NewScope(nil, nil)
	a := &Symbol{Name: "X", Kind: SymbolPackage}
	b := &Symbol{Name: "X", Kind: SymbolNamespace}
	root.Define("X", a)
	root.Define("X", b)

	all := root.LookupLocalAll("X")
	if len(all) != 2 {
		t.Fatalf("LookupLocalAll(X) len = %d, want 2", len(all))
	}
	// LookupLocal returns the first-defined symbol.
	got, ok := root.LookupLocal("X")
	if !ok || got != a {
		t.Fatalf("LookupLocal(X) = %v; want first-defined a", got)
	}
}
