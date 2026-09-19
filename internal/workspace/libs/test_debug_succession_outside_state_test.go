package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestSuccessionOutsideState(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "succession do then x outside state",
			code: `behavior B { succession do then x; }`,
		},
		{
			name: "succession [1] do.start then x.end outside state",
			code: `behavior B { succession [1] do.start then x.end; }`,
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
