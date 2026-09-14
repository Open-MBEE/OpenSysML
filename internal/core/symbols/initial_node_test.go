package symbols

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"testing"
)

// An action body's `first start then off;` is an InitialNodeMember whose name
// `start` is indexed as a member of the body.
func TestInitialNodeIndexing(t *testing.T) {
	src := `package test {
	action def S {
		first start then off;
		action off;
	}
}`

	file := parser.New(source.New("test", []byte(src))).ParseFile()
	idx := NewIndex()
	idx.AddDocument("test", file)

	// Find action def S scope
	rootScope := idx.DocumentRoot("test")
	if rootScope == nil {
		t.Fatal("Root scope not found")
	}

	// Find package
	t.Logf("Root children: %d", len(rootScope.Children()))
	for i, child := range rootScope.Children() {
		t.Logf("  [%d] %T", i, child.Node())
	}

	var pkgScope *Scope
	for _, child := range rootScope.Children() {
		if pkg, ok := child.Node().(*ast.Package); ok {
			t.Logf("Found package: %s", pkg.Ident.Name)
			pkgScope = child
			break
		}
	}
	if pkgScope == nil {
		t.Fatal("Package scope not found")
	}

	// Find action def S
	t.Logf("Package children: %d", len(pkgScope.Children()))
	for i, child := range pkgScope.Children() {
		node := child.Node()
		t.Logf("  [%d] %T", i, node)
		if usage, ok := node.(*ast.Usage); ok {
			t.Logf("      name=%s, kind=%v", usage.Ident.Name, usage.Kind)
		}
	}

	var actionScope *Scope
	for _, child := range pkgScope.Children() {
		if def, ok := child.Node().(*ast.Definition); ok {
			t.Logf("Found definition: %s", def.Ident.Name)
			actionScope = child
			break
		}
	}
	if actionScope == nil {
		t.Fatal("Action scope not found")
	}

	// Check AST members directly
	if actionDef, ok := actionScope.Node().(*ast.Definition); ok {
		t.Logf("Action AST members: %d", len(actionDef.Members))
		for i, m := range actionDef.Members {
			t.Logf("  [%d] %T", i, m)
			if init, ok := m.(*ast.InitialNode); ok {
				t.Logf("      InitialNode name=%s", init.Name)
			}
			if usage, ok := m.(*ast.Usage); ok {
				t.Logf("      Usage name=%s, kind=%v, members=%d", usage.Ident.Name, usage.Kind, len(usage.Members))
				for j, um := range usage.Members {
					t.Logf("        [%d] %T", j, um)
					if init2, ok2 := um.(*ast.InitialNode); ok2 {
						t.Logf("            InitialNode name=%s", init2.Name)
					}
				}
			}
		}
	}

	// Check if "start" is registered
	names := actionScope.MemberNames()
	t.Logf("Action members: %v", names)

	startSym, _ := actionScope.LookupLocal("start")
	if startSym == nil {
		t.Errorf("'start' not found in action scope")
	} else {
		t.Logf("Found 'start': %T", startSym.Decl)
	}
}
