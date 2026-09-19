package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestNewDocumentParses(t *testing.T) {
	d := newDocument("a.sysml", []byte("package P { namespace N; }"), 1)
	if d.Name != "a.sysml" {
		t.Fatalf("Name = %q, want a.sysml", d.Name)
	}
	if d.Version != 1 {
		t.Fatalf("Version = %d, want 1", d.Version)
	}
	if d.AST == nil {
		t.Fatal("AST is nil")
	}
	if len(d.AST.Members) != 1 {
		t.Fatalf("len(Members) = %d, want 1", len(d.AST.Members))
	}
	if len(d.ParseDiagnostics) != 0 {
		t.Fatalf("ParseDiagnostics = %d, want 0", len(d.ParseDiagnostics))
	}
	if d.Scope == nil {
		t.Fatal("Scope is nil")
	}
	if _, ok := d.Scope.LookupLocal("P"); !ok {
		t.Fatal("P not in scope")
	}
	var _ = ast.Node(nil)
}

func TestNewDocumentReportsParseDiagnostics(t *testing.T) {
	d := newDocument("bad.sysml", []byte("package"), 1)
	if len(d.ParseDiagnostics) == 0 {
		t.Fatal("expected parse diagnostics for incomplete package")
	}
	if d.AST == nil {
		t.Fatal("AST should still be non-nil after recovery")
	}
}
