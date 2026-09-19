package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// exposeIn parses a view usage whose body holds a single expose declaration and
// returns the Import node the parser produced for it.
func exposeIn(t *testing.T, decl string) *ast.Import {
	t.Helper()
	src := source.New("expose.sysml", []byte("view v { "+decl+" }"))
	p := New(src)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("%s", d.Message)
		}
		t.FailNow()
	}
	view := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
	imp, ok := view.Members[0].(*ast.Import)
	if !ok {
		t.Fatalf("expose parsed as %T, want *ast.Import", view.Members[0])
	}
	return imp
}

// An Expose is an Import (SysML v2 8.3.26.2): MembershipExpose specializes
// MembershipImport and NamespaceExpose specializes NamespaceImport, so the
// wildcard tail selects the import kind exactly as it does for `import`.
func TestParseExposeImportKind(t *testing.T) {
	tests := []struct {
		decl      string
		kind      ast.ImportKind
		recursive bool
	}{
		{"expose X;", ast.ImportMembership, false},
		{"expose X::*;", ast.ImportNamespace, false},
		{"expose X::**;", ast.ImportMembership, true},
		{"expose X::*::**;", ast.ImportNamespace, true},
	}
	for _, tt := range tests {
		t.Run(tt.decl, func(t *testing.T) {
			imp := exposeIn(t, tt.decl)
			if imp.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", imp.Kind, tt.kind)
			}
			if imp.IsRecursive != tt.recursive {
				t.Errorf("IsRecursive = %t, want %t", imp.IsRecursive, tt.recursive)
			}
		})
	}
}

// An Expose always imports all elements regardless of visibility
// (isImportAll = true) and always has protected visibility
// (validateExposeIsImportAll / validateExposeVisibility, SysML v2 8.3.26.2).
func TestParseExposeIsImportAllAndProtected(t *testing.T) {
	for _, decl := range []string{"expose X;", "expose X::*;", "expose X::**;"} {
		imp := exposeIn(t, decl)
		if !imp.IsExpose {
			t.Errorf("%s: IsExpose = false, want true", decl)
		}
		if !imp.IsAll {
			t.Errorf("%s: IsAll = false, want true (isImportAll)", decl)
		}
		if imp.Visibility != ast.VisibilityProtected {
			t.Errorf("%s: Visibility = %v, want protected", decl, imp.Visibility)
		}
	}
}

func TestParseExposeFilterExpression(t *testing.T) {
	imp := exposeIn(t, "expose X::**[@SysML::PartUsage];")
	if imp.Kind != ast.ImportMembership || !imp.IsRecursive {
		t.Errorf("Kind = %v, IsRecursive = %t; want membership/recursive", imp.Kind, imp.IsRecursive)
	}
	if imp.FilterExpr == nil {
		t.Error("FilterExpr = nil, want the parsed filter expression")
	}
}

// A plain import keeps the kinds it always had: the shared tail parser must not
// change import behavior.
func TestParseImportKindUnchanged(t *testing.T) {
	tests := []struct {
		decl      string
		kind      ast.ImportKind
		recursive bool
		all       bool
	}{
		{"import X;", ast.ImportMembership, false, false},
		{"import X::*;", ast.ImportNamespace, false, false},
		{"import X::**;", ast.ImportMembership, true, false},
		{"import X::*::**;", ast.ImportNamespace, true, false},
		{"import all X::*;", ast.ImportNamespace, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.decl, func(t *testing.T) {
			src := source.New("import.sysml", []byte(tt.decl))
			p := New(src)
			root := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				for _, d := range p.Diagnostics {
					t.Errorf("%s", d.Message)
				}
				t.FailNow()
			}
			imp, ok := root.Members[0].(*ast.Import)
			if !ok {
				t.Fatalf("import parsed as %T, want *ast.Import", root.Members[0])
			}
			if imp.Kind != tt.kind || imp.IsRecursive != tt.recursive || imp.IsAll != tt.all {
				t.Errorf("Kind = %v, IsRecursive = %t, IsAll = %t; want %v/%t/%t",
					imp.Kind, imp.IsRecursive, imp.IsAll, tt.kind, tt.recursive, tt.all)
			}
			if imp.IsExpose {
				t.Error("IsExpose = true for an import declaration")
			}
		})
	}
}
