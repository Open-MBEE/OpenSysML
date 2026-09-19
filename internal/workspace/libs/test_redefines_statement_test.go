package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestRedefinesStatement(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "simple redefines with literal",
			input: `
				feature spaceTimeEnclosedPoints {
					redefines innerSpaceDimension = 0;
				}
			`,
		},
		{
			name: "redefines with feature chain",
			input: `
				feature myFeature {
					redefines parent.value = 100;
				}
			`,
		},
		{
			name: "redefines with expression",
			input: `
				feature calc {
					redefines result = x + y;
				}
			`,
		},
		{
			name: "mixed with other members",
			input: `
				feature spaceTimeEnclosedPoints {
					redefines innerSpaceDimension = 0;
					binding [1] startShot = [1] endShot;
					attribute count: Integer;
				}
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := source.New("test.kerml", []byte(tt.input))
			p := parser.New(src)
			root := p.ParseFile()

			diags := p.Diagnostics
			for _, d := range diags {
				t.Logf("[Offset %d] %s", d.Span.Offset, d.Message)
			}

			if len(diags) > 0 {
				t.Fatalf("Expected clean parse, got %d errors", len(diags))
			}

			if root == nil {
				t.Fatal("root is nil")
			}
		})
	}
}
