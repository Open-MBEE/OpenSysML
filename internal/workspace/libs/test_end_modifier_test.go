package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Test if end modifier detection works
func TestEndModifier(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors int
	}{
		{
			name: "simple end feature",
			input: `
				connector SelfLink {
					end feature thisThing: Anything;
				}
			`,
			errors: 0,
		},
		{
			name: "end with short name",
			input: `
				connector SelfLink {
					end self2 [1] feature sameThing: Anything;
				}
			`,
			errors: 0, // should work after fix
		},
		{
			name: "end with mult no name",
			input: `
				flow {
					end [1] feature transferSource references source;
				}
			`,
			errors: 0, // different pattern - may still fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()

			t.Logf("Got %d diagnostics (expected %d)", len(p.Diagnostics), tt.errors)
			for _, d := range p.Diagnostics {
				t.Logf("  - %s (offset %d)", d.Message, d.Span.Offset)
			}

			if len(p.Diagnostics) != tt.errors {
				t.Errorf("Expected %d errors, got %d", tt.errors, len(p.Diagnostics))
			}
		})
	}
}
