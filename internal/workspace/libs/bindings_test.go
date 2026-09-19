package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestBindKeywordSupport tests that "bind" works as shorthand for "binding"
func TestBindKeywordSupport(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "simple bind statement",
			code: `
package Test {
	part Foo {
		bind payload = accepter.payload;
	}
}
`,
		},
		{
			name: "bind with feature chain",
			code: `
package Test {
	part Foo {
		bind receiver = accepter.receiver;
	}
}
`,
		},
		{
			name: "multiple bind statements",
			code: `
package Test {
	state MyState {
		bind payload = accepter.payload;
		bind receiver = accepter.receiver;
	}
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.sysml", []byte(tt.code)))
			root := p.ParseFile()

			if len(p.Diagnostics) > 0 {
				t.Logf("Diagnostics:")
				for _, d := range p.Diagnostics {
					t.Logf("  %s", d.Message)
				}
				t.Fatalf("Expected clean parse, got %d diagnostics", len(p.Diagnostics))
			}

			// Verify AST structure
			if len(root.Members) == 0 {
				t.Fatal("No members parsed")
			}

			// Check that bind created UsageBinding
			pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
			if len(pkg.Members) == 0 {
				t.Fatal("No package members")
			}

			part := pkg.Members[0].(*ast.Membership).Member.(*ast.Usage)
			if len(part.Members) == 0 {
				t.Fatal("No part members")
			}

			bind := part.Members[0].(*ast.Membership).Member.(*ast.Usage)
			if bind.Kind != ast.UsageBinding {
				t.Errorf("Expected UsageBinding, got %v", bind.Kind)
			}
		})
	}
}
