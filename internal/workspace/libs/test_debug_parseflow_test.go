package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestDebugParseFlow(t *testing.T) {
	// Simplest case: just succession keyword and start of pattern
	tests := []struct {
		name string
		code string
	}{
		{
			name: "succession [mult] do then x",
			code: `state S { succession [1] do then x; }`,
		},
		{
			name: "succession do then x",
			code: `state S { succession do then x; }`,
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

			for _, d := range p.Diagnostics {
				t.Errorf("offset %d: %s", d.Span.Offset, d.Message)
			}
		})
	}
}
