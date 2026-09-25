package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestChoicePseudostateParsing(t *testing.T) {
	src := source.New("test.sysml", []byte(`
package Test {
	state def Controller {
		state Idle;
		state Active;
		choice checkPriority;
		
		transition first Idle then checkPriority;
		transition first checkPriority if (priority > 5) then Active;
	}
}
`))

	p := New(src)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Diagnostic: %s", d.Message)
		}
		t.Fatalf("Expected 0 diagnostics, got %d", len(p.Diagnostics))
	}

	// Navigate to state def
	pkg, ok := root.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected membership, got %T", root.Members[0])
	}

	pkgNode, ok := pkg.Member.(*ast.Package)
	if !ok {
		t.Fatalf("Expected Package, got %T", pkg.Member)
	}

	stateMembership, ok := pkgNode.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected state membership, got %T", pkgNode.Members[0])
	}

	stateDef, ok := stateMembership.Member.(*ast.Definition)
	if !ok {
		t.Fatalf("Expected Definition, got %T", stateMembership.Member)
	}

	// Find choice pseudostate in members
	var foundChoice *ast.PseudostateNode
	for _, member := range stateDef.Members {
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		if ps, ok := member.(*ast.PseudostateNode); ok {
			if ps.Kind == ast.PseudostateChoice && ps.Name == "checkPriority" {
				foundChoice = ps
				break
			}
		}
	}

	if foundChoice == nil {
		t.Fatal("Expected to find choice pseudostate 'checkPriority'")
	}

	t.Logf("Found choice pseudostate: %s", foundChoice.Name)
}

func TestJunctionPseudostateParsing(t *testing.T) {
	src := source.New("test.sysml", []byte(`
package Test {
	state def Monitor {
		state Nominal;
		state Warning;
		state Critical;
		junction statusEval;
		
		transition first statusEval if (temp < 50) then Nominal;
		transition first statusEval if (temp >= 50 and temp < 100) then Warning;
		transition first statusEval if (temp >= 100) then Critical;
	}
}
`))

	p := New(src)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Diagnostic: %s", d.Message)
		}
		t.Fatalf("Expected 0 diagnostics, got %d", len(p.Diagnostics))
	}

	// Navigate to state def
	pkg2, ok := root.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected membership, got %T", root.Members[0])
	}

	pkgNode2, ok := pkg2.Member.(*ast.Package)
	if !ok {
		t.Fatalf("Expected Package, got %T", pkg2.Member)
	}

	stateMembership2, ok := pkgNode2.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected state membership, got %T", pkgNode2.Members[0])
	}

	stateDef2, ok := stateMembership2.Member.(*ast.Definition)
	if !ok {
		t.Fatalf("Expected Definition, got %T", stateMembership2.Member)
	}

	// Find junction pseudostate in members
	var foundJunction *ast.PseudostateNode
	for _, member := range stateDef2.Members {
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		if ps, ok := member.(*ast.PseudostateNode); ok {
			if ps.Kind == ast.PseudostateJunction && ps.Name == "statusEval" {
				foundJunction = ps
				break
			}
		}
	}

	if foundJunction == nil {
		t.Fatal("Expected to find junction pseudostate 'statusEval'")
	}

	t.Logf("Found junction pseudostate: %s", foundJunction.Name)
}
