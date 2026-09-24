package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestConnectionConnect(t *testing.T) {
	code := `connection :MatesWith connect [1] be to [1] be;`

	p := New(source.New("test.kerml", []byte(code)))
	_ = p.ParseFile()
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("Offset %d: %s", d.Span.Offset, d.Message)
			// Show context
			if d.Span.Offset < len(code) {
				start := d.Span.Offset
				if start > 10 {
					start -= 10
				} else {
					start = 0
				}
				end := d.Span.Offset + 20
				if end > len(code) {
					end = len(code)
				}
				t.Logf("  Context: %q", code[start:end])
				t.Logf("  Current token would be: %q", code[d.Span.Offset:d.Span.Offset+5])
			}
		}
	}
}
