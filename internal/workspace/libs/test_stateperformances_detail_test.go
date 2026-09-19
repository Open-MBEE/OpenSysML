package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestStatePerformancesDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/StatePerformances.kerml")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	p := parser.New(source.New("StatePerformances.kerml", data))
	_ = p.ParseFile()

	t.Logf("StatePerformances.kerml: %d diagnostics", len(p.Diagnostics))

	code := string(data)
	for i, d := range p.Diagnostics {
		t.Logf("\n=== Error %d: %s ===", i+1, d.Message)
		t.Logf("Offset: %d", d.Span.Offset)

		// Show context around error
		start := d.Span.Offset - 100
		if start < 0 {
			start = 0
		}
		end := d.Span.Offset + 100
		if end > len(code) {
			end = len(code)
		}

		context := code[start:end]
		markerPos := d.Span.Offset - start

		t.Logf("Context:\n%s", context)
		t.Logf("Marker: %s^", string(make([]byte, markerPos)))

		if d.Span.Offset < len(code) {
			t.Logf("Character at offset: %q", code[d.Span.Offset])
		}
	}
}
