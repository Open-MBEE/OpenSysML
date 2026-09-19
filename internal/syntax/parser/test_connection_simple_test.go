package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestConnectionSimple(t *testing.T) {
	// Test even simpler case
	code := `connection connect [1] be to [1] be;`

	p := New(source.New("test.kerml", []byte(code)))
	_ = p.ParseFile()
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("Offset %d: %s", d.Span.Offset, d.Message)
		}
	}
}

func TestConnectionTyped(t *testing.T) {
	// With typing relationship
	code := `connection :MatesWith connect [1] be to [1] be;`

	p := New(source.New("test.kerml", []byte(code)))
	_ = p.ParseFile()
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("Offset %d: %s", d.Span.Offset, d.Message)
		}
	}
}
