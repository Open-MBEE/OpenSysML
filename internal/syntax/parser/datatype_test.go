package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestDatatypePatterns verifies parser treats datatype uniformly as usage keyword.
// Phase 4: Semantic classification (def vs usage) is deferred to symbol builder/semantics.
func TestDatatypePatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKind string
	}{
		{
			name:     "datatype_with_specializes",
			input:    "datatype MyType specializes BaseType;",
			wantKind: "attribute", // Parser creates usage uniformly
		},
		{
			name:     "datatype_standalone",
			input:    "datatype MyType;",
			wantKind: "attribute", // Parser creates usage uniformly
		},
		{
			name:     "datatype_with_typing",
			input:    "datatype myValue : MyType;",
			wantKind: "attribute", // Parser creates usage uniformly
		},
		{
			name:     "datatype_with_body",
			input:    "datatype MyType { attribute x; }",
			wantKind: "attribute", // Parser creates usage uniformly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "package Test { " + tt.input + " }"
			p := New(source.New("test.sysml", []byte(src)))
			root := p.ParseFile()

			if len(p.Diagnostics) > 0 {
				t.Fatalf("parse errors: %v", p.Diagnostics)
			}

			// Unwrap nested structure: RootNamespace -> Membership -> Package -> Membership -> Definition/Usage
			member := root.Members[0]
			if m, ok := member.(*ast.Membership); ok {
				member = m.Member
			}

			pkg, ok := member.(*ast.Package)
			if !ok {
				t.Fatalf("expected Package, got %T", member)
			}

			if len(pkg.Members) != 1 {
				t.Fatalf("expected 1 member, got %d", len(pkg.Members))
			}

			pkgMember := pkg.Members[0]
			// Unwrap Membership if present
			if m, ok := pkgMember.(*ast.Membership); ok {
				pkgMember = m.Member
			}

			// Parser should create Usage uniformly for all datatype patterns
			usage, ok := pkgMember.(*ast.Usage)
			if !ok {
				t.Fatalf("expected Usage, got %T", pkgMember)
			}
			if usage.Kind.String() != tt.wantKind {
				t.Errorf("expected kind %s, got %s", tt.wantKind, usage.Kind)
			}
		})
	}
}
