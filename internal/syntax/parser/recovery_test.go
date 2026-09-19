package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestRecoverBadMemberThenGood(t *testing.T) {
	p := newParser("@@@ package P;")
	root := p.ParseFile()
	if len(root.Members) != 2 {
		t.Fatalf("got %d members, want 2", len(root.Members))
	}
	if _, ok := root.Members[0].(*ast.ErrorNode); !ok {
		t.Errorf("member[0] = %T, want *ast.ErrorNode", root.Members[0])
	}
	if _, ok := root.Members[1].(*ast.Membership); !ok {
		t.Errorf("member[1] = %T, want *ast.Membership", root.Members[1])
	}
}

func TestRecoverBadMemberNoSemicolon(t *testing.T) {
	p := newParser("garble package P;")
	root := p.ParseFile()
	if len(root.Members) != 2 {
		t.Fatalf("got %d members, want 2", len(root.Members))
	}
	if _, ok := root.Members[1].(*ast.Membership); !ok {
		t.Errorf("member[1] = %T, want *ast.Membership", root.Members[1])
	}
}

func TestRecoverMissingCloseBrace(t *testing.T) {
	p := newParser("package P { package Q;")
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("got %d members, want 1", len(root.Members))
	}
	if len(p.Diagnostics) == 0 {
		t.Errorf("expected diagnostics for missing '}'")
	}
}

func TestRecoverAlwaysTerminates(t *testing.T) {
	p := newParser("} ] ) :: :: ;;; @@@ package X;")
	root := p.ParseFile()
	if root == nil {
		t.Fatal("ParseFile returned nil")
	}
}
