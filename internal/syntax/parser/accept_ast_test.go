package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestAcceptNodeWithoutName covers `action accept …`, the accept node naming no
// node of its own (SysML.xtext AcceptNodeDeclaration: the declaration after the
// keyword is optional).
func TestAcceptNodeWithoutName(t *testing.T) {
	p := New(source.New("test", []byte(`package test {
		item def Scene;
		action takePicture {
			action accept scene : Scene;
		}
	}`)))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", p.Diagnostics)
	}
	for _, w := range p.Warnings {
		if w.Code == codeReservedKeywordName {
			t.Errorf("`accept` was read as the node's name: %v", w)
		}
	}

	pkg := file.Members[0].(*ast.Membership).Member.(*ast.Package)
	action := pkg.Members[1].(*ast.Membership).Member.(*ast.Usage)
	node, ok := action.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if !ok {
		t.Fatalf("expected an action usage, got %T", action.Members[0].(*ast.Membership).Member)
	}
	if node.Ident.Name != "" {
		t.Errorf("expected an unnamed accept node, got name %q", node.Ident.Name)
	}
	payload, ok := node.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if !ok {
		t.Fatalf("expected a payload parameter, got %T", node.Members[0].(*ast.Membership).Member)
	}
	if !payload.IsAccept || payload.Ident.Name != "scene" {
		t.Errorf("expected the accept payload `scene`, got %q (IsAccept=%v)", payload.Ident.Name, payload.IsAccept)
	}
}

func TestAcceptActionAST(t *testing.T) {
	src := `package test {
		item def Scene;
		
		action takePicture {
			action trigger accept scene : Scene;
		}
	}`

	file := New(source.New("test", []byte(src))).ParseFile()

	t.Logf("Root members: %d", len(file.Members))
	for i, m := range file.Members {
		if memb, ok := m.(*ast.Membership); ok {
			t.Logf("  Member %d: %T", i, memb.Member)
		}
	}

	// Navigate to action takePicture
	pkg := file.Members[0].(*ast.Membership).Member.(*ast.Package)
	t.Logf("Package members: %d", len(pkg.Members))
	for i, m := range pkg.Members {
		if memb, ok := m.(*ast.Membership); ok {
			t.Logf("  Member %d: %T", i, memb.Member)
		}
	}

	if len(pkg.Members) < 2 {
		t.Fatalf("Expected at least 2 members in package, got %d", len(pkg.Members))
	}

	takePictureMemb := pkg.Members[1].(*ast.Membership)
	takePicture := takePictureMemb.Member.(*ast.Usage)

	// Check nested action trigger
	triggerMemb := takePicture.Members[0].(*ast.Membership)
	trigger := triggerMemb.Member

	t.Logf("Nested action type: %T", trigger)
	if usage, ok := trigger.(*ast.Usage); ok {
		t.Logf("  Name: %s", usage.Ident.Name)
		t.Logf("  Kind: %v", usage.Kind)
		t.Logf("  Members count: %d", len(usage.Members))
		for i, m := range usage.Members {
			if memb, ok := m.(*ast.Membership); ok {
				t.Logf("    Member %d: %T", i, memb.Member)
				if innerUsage, ok := memb.Member.(*ast.Usage); ok {
					t.Logf("      Name: %s", innerUsage.Ident.Name)
					t.Logf("      Kind: %v", innerUsage.Kind)
					t.Logf("      Direction: %v", innerUsage.Direction)
					t.Logf("      IsAccept: %v", innerUsage.IsAccept)
				}
			}
		}
	}
}

// An accept node may be identified by a short name and a name both
// (SysML.xtext Identification), which the accept lookahead must see past.
func TestAcceptNodeWithShortNameAndName(t *testing.T) {
	p := New(source.New("test", []byte(`package test {
		item def Scene;
		action takePicture {
			action <s> shutter accept scene : Scene;
		}
	}`)))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", p.Diagnostics)
	}

	pkg := file.Members[0].(*ast.Membership).Member.(*ast.Package)
	action := pkg.Members[1].(*ast.Membership).Member.(*ast.Usage)
	node := action.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if node.Ident.ShortName != "s" || node.Ident.Name != "shutter" {
		t.Fatalf("accept node identified as <%s> %q", node.Ident.ShortName, node.Ident.Name)
	}
	payload, ok := node.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if !ok {
		t.Fatalf("expected a payload parameter, got %T", node.Members[0].(*ast.Membership).Member)
	}
	if !payload.IsAccept || payload.Ident.Name != "scene" {
		t.Errorf("expected the accept payload `scene`, got %q (IsAccept=%v)", payload.Ident.Name, payload.IsAccept)
	}
}
