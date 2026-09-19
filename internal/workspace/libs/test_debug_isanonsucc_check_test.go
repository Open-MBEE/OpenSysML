package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestIsAnonSuccessionCheck(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		wantNoErrors bool
	}{
		{
			name:         "succession do then x - should parse cleanly",
			code:         `state S { succession do then x; }`,
			wantNoErrors: true,
		},
		{
			name:         "succession [1] do then x - should parse cleanly",
			code:         `state S { succession [1] do then x; }`,
			wantNoErrors: true,
		},
		{
			name:         "succession do.start then x - should parse cleanly",
			code:         `state S { succession do.start then x; }`,
			wantNoErrors: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := source.New("test.kerml", []byte(tt.code))
			p := parser.New(file)
			root := p.ParseFile()

			if root == nil {
				t.Fatal("ParseFile returned nil")
			}

			hasErrors := len(p.Diagnostics) > 0
			if tt.wantNoErrors && hasErrors {
				t.Errorf("Expected no errors, got:")
				for _, d := range p.Diagnostics {
					t.Errorf("  offset %d: %s", d.Span.Offset, d.Message)
				}
			} else if !tt.wantNoErrors && !hasErrors {
				t.Error("Expected errors, got none")
			}
		})
	}
}
