package symbols

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"testing"
)

// An action body's one-ended `first start;` is an InitialNodeMember whose name
// `start` is indexed as a member of the body.
func TestInitialNodeIndexing(t *testing.T) {
	src := `package test {
	action def S {
		first start;
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
				t.Logf("      InitialNode name=%s", init.Name())
			}
			if usage, ok := m.(*ast.Usage); ok {
				t.Logf("      Usage name=%s, kind=%v, members=%d", usage.Ident.Name, usage.Kind, len(usage.Members))
				for j, um := range usage.Members {
					t.Logf("        [%d] %T", j, um)
					if init2, ok2 := um.(*ast.InitialNode); ok2 {
						t.Logf("            InitialNode name=%s", init2.Name())
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

// A one-ended `first a;` declares a label under a's name; an action body's
// `first a then b;` names the source of a succession and declares nothing, and
// a state body's `first start then off;` is a succession usage, no label either.
func TestFirstNamesASourceOnlyInATwoEndedActionForm(t *testing.T) {
	root := build(t, `package P {
	action def Marked {
		first prep;
		action prep;
	}
	action def Sequenced {
		action prep;
		first prep then launch;
		action launch;
		first missing then launch;
	}
	state def Machine {
		first start then off;
		state off;
	}
}`)
	pkg, _ := root.LookupLocal("P")
	labels := func(owner string, name string) int {
		def, ok := pkg.Scope.LookupLocal(owner)
		if !ok {
			t.Fatalf("%s is not indexed", owner)
		}
		n := 0
		for _, sym := range def.Scope.LookupLocalAll(name) {
			if _, label := sym.Decl.(*ast.InitialNode); label {
				n++
			}
		}
		return n
	}
	if got := labels("Marked", "prep"); got != 1 {
		t.Errorf("`first prep;` should declare one label, found %d", got)
	}
	if got := labels("Sequenced", "prep"); got != 0 {
		t.Errorf("`first prep then launch;` should declare no label, found %d", got)
	}
	if got := labels("Sequenced", "missing"); got != 0 {
		t.Errorf("`first missing then launch;` should declare no label, found %d", got)
	}
	if got := labels("Machine", "start"); got != 0 {
		t.Errorf("a state machine's `first start then off;` should declare no label, found %d", got)
	}
	for def, want := range map[string]bool{"Sequenced": false, "Machine": true} {
		sym, _ := pkg.Scope.LookupLocal(def)
		if got := InStateMachine(sym.Scope); got != want {
			t.Errorf("InStateMachine(%s) = %v, want %v", def, got, want)
		}
	}
}
