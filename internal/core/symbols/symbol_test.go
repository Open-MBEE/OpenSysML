package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestSymbolKindString(t *testing.T) {
	if SymbolPackage.String() != "package" {
		t.Fatalf("SymbolPackage.String() = %q, want %q", SymbolPackage.String(), "package")
	}
	if SymbolNamespace.String() != "namespace" {
		t.Fatalf("SymbolNamespace.String() = %q, want %q", SymbolNamespace.String(), "namespace")
	}
	if SymbolAlias.String() != "alias" {
		t.Fatalf("SymbolAlias.String() = %q, want %q", SymbolAlias.String(), "alias")
	}
}

func TestSymbolFields(t *testing.T) {
	pkg := &ast.Package{}
	pkg.NodeSpan = source.Span{Offset: 3, Len: 7}
	s := &Symbol{
		Name:       "P",
		Kind:       SymbolPackage,
		Decl:       pkg,
		Visibility: ast.VisibilityPublic,
		DeclSpan:   source.Span{Offset: 3, Len: 7},
	}
	if s.Name != "P" || s.Kind != SymbolPackage {
		t.Fatalf("unexpected symbol fields: %+v", s)
	}
	if s.DeclSpan.Offset != 3 || s.DeclSpan.Len != 7 {
		t.Fatalf("unexpected DeclSpan: %+v", s.DeclSpan)
	}
	var _ ast.Node = s.Decl
}

func TestSymbolOwner(t *testing.T) {
	var none *Symbol
	if none.Owner() != nil {
		t.Fatal("nil symbol: want nil owner")
	}
	root := &Symbol{Name: "Root"}
	if root.Owner() != nil {
		t.Fatal("symbol without owner scope: want nil owner")
	}
	body := NewScope(nil, nil)
	if (&Symbol{Name: "x", OwnerScope: body}).Owner() != nil {
		t.Fatal("scope without owner: want nil owner")
	}
	body.SetOwner(root)
	if got := (&Symbol{Name: "x", OwnerScope: body}).Owner(); got != root {
		t.Fatalf("owner = %v, want Root", got)
	}
}
