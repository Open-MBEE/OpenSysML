package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestParseSuccession(t *testing.T) {
	src := `
behavior def Test {
	succession [1] ifTest then [0..1] thenClause;
}
`
	sf := source.New("test.sysml", []byte(src))
	p := New(sf)
	file := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Diagnostic: %s [offset %d, len %d]", d.Message, d.Span.Offset, d.Span.Len)
		}
		t.Fatalf("Expected no errors, got %d", len(p.Diagnostics))
	}

	if len(file.Members) != 1 {
		t.Fatalf("Expected 1 namespace member, got %d", len(file.Members))
	}

	mem, ok := file.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected membership, got %T", file.Members[0])
	}

	def, ok := mem.Member.(*ast.Definition)
	if !ok {
		t.Fatalf("Expected definition, got %T", mem.Member)
	}

	if def.Kind != ast.DefBehavior {
		t.Fatalf("Expected DefBehavior, got %v", def.Kind)
	}

	if len(def.Members) != 1 {
		t.Fatalf("Expected 1 body member, got %d", len(def.Members))
	}

	mem2, ok := def.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected membership, got %T", def.Members[0])
	}

	usage, ok := mem2.Member.(*ast.Usage)
	if !ok {
		t.Fatalf("Expected usage, got %T", mem2.Member)
	}

	if usage.Kind != ast.UsageSuccession {
		t.Errorf("Expected UsageSuccession, got %v", usage.Kind)
	}

	if len(usage.ConnectorEnds) != 2 {
		t.Fatalf("Expected 2 connector ends, got %d", len(usage.ConnectorEnds))
	}

	t.Log("Succession parsed successfully with 2 connector ends!")
}
