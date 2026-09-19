package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestBindingMultNameMultDebug(t *testing.T) {
	code := `binding [1] bind [0..*] base.edges = [0..*] be;`
	//        0123456789012345678901234567890123456789
	//        0         1         2         3

	p := New(source.New("test.sysml", []byte(code)))
	_ = p.ParseFile()
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Offset %d: %s", d.Span.Offset, d.Message)
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
			}
		}
		t.Fail()
	}
}
