package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestAnonymousFeatureWithMultiplicity(t *testing.T) {
	code := `
attribute def Test {
	ref explanation : Anything [0..1] {
		doc /* comment */
	}
}
`
	sf := source.New("test.sysml", []byte(code))
	p := New(sf)
	file := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Diagnostic: %s at offset %d", d.Message, d.Span.Offset)
		}
		t.Fatalf("Expected no errors, got %d", len(p.Diagnostics))
	}

	// Check structure: file.Members[0] is Membership with Definition member
	mem0, ok := file.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected Membership, got %T", file.Members[0])
	}

	attrDef, ok := mem0.Member.(*ast.Definition)
	if !ok {
		t.Fatalf("Expected Definition, got %T", mem0.Member)
	}

	if len(attrDef.Members) == 0 {
		t.Fatal("Expected members in attribute def")
	}

	usageMem, ok := attrDef.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected Membership, got %T", attrDef.Members[0])
	}

	usage, ok := usageMem.Member.(*ast.Usage)
	if !ok {
		t.Fatalf("Expected Usage, got %T", usageMem.Member)
	}

	if usage.Ident.Name != "explanation" {
		t.Errorf("Expected name 'explanation', got %q", usage.Ident.Name)
	}

	if !usage.IsReference {
		t.Error("Expected IsReference=true")
	}

	if usage.Multiplicity == nil {
		t.Fatal("Expected multiplicity")
	}

	if !usage.HasBody {
		t.Error("Expected body")
	}

	if len(usage.Members) == 0 {
		t.Error("Expected doc in body")
	}

	t.Logf("Anonymous feature with multiplicity parsed correctly")
}
