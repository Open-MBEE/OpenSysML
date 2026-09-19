package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestViewsDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Views.sysml")
	if err != nil {
		t.Fatalf("Failed to load Views.sysml: %v", err)
	}

	p := parser.New(source.New("Views.sysml", data))
	_ = p.ParseFile()

	t.Logf("Views.sysml: %d diagnostics", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		offset := d.Span.Offset
		char := ""
		if offset < len(data) {
			char = string(data[offset])
		}

		// Get context
		start := offset - 40
		if start < 0 {
			start = 0
		}
		end := offset + 40
		if end > len(data) {
			end = len(data)
		}
		context := string(data[start:end])

		t.Logf("\n%d. Offset %d (char=%q): %s", i+1, offset, char, d.Message)
		t.Logf("   Context: %q", context)
	}
}
