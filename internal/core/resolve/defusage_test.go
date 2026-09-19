package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestResolveDefinitionSpecializesResolves(t *testing.T) {
	r := resolveDoc(t, "<t>", "part def Vehicle; part def Car specializes Vehicle;")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

func TestResolveDefinitionSpecializesUnresolved(t *testing.T) {
	r := resolveDoc(t, "<t>", "part def Car specializes Missing;")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved-target diagnostic")
	}
}

func TestResolveUsageValueReference(t *testing.T) {
	r := resolveDoc(t, "<t>", "part def Car { attribute base; attribute mass = base; }")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

func TestResolveUsageValueUnresolved(t *testing.T) {
	r := resolveDoc(t, "<t>", "part def Car { attribute mass = undefinedRef; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved value-reference diagnostic")
	}
}

func TestResolveNestedUsageTyping(t *testing.T) {
	r := resolveDoc(t, "<t>", "part def Engine; part def Car { part engine : Engine; }")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
	_ = ast.RelTyping // keep ast imported
}
