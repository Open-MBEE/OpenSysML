package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestParseStepUsage(t *testing.T) {
	input := `step removeStep;`

	sf := source.New("test.sysml", []byte(input))
	p := New(sf)

	// Pre-parse check
	if !p.atDefUsageStart() {
		t.Error("atDefUsageStart() returned false before parsing")
	}

	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Log("Diagnostics:")
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			t.Logf("  %s [near: %q at offset %d]", d.Message, text, d.Span.Offset)
		}
		t.FailNow()
	}

	if len(root.Members) != 1 {
		t.Fatalf("Expected 1 member, got %d", len(root.Members))
	}

	member, ok := root.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected Membership, got %T", root.Members[0])
	}

	step, ok := member.Member.(*ast.Usage)
	if !ok {
		t.Fatalf("Expected Usage, got %T", member.Member)
	}

	if step.Kind != ast.UsageStep {
		t.Errorf("Expected UsageStep, got %v", step.Kind)
	}

	if step.Ident.Name != "removeStep" {
		t.Errorf("Expected name 'removeStep', got %s", step.Ident.Name)
	}

	t.Logf("Successfully parsed step usage: %s", step.Ident.Name)
}
