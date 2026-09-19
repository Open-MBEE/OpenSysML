package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestOccurrencesRedefinesStatement(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/Occurrences.kerml")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	sf := source.New("Occurrences.kerml", data)
	p := parser.New(sf)
	root := p.ParseFile()

	if root == nil {
		t.Fatal("root is nil")
	}

	diags := p.Diagnostics
	t.Logf("Total diagnostics: %d", len(diags))

	// Focus on "redefines innerSpaceDimension = 0" line
	for _, d := range diags {
		if d.Span.Offset >= 5900 && d.Span.Offset <= 6100 {
			// Get context
			start := d.Span.Offset - 50
			if start < 0 {
				start = 0
			}
			end := d.Span.Offset + 50
			if end > len(data) {
				end = len(data)
			}
			context := string(data[start:end])
			t.Logf("[Offset %d] %s\n  Context: %q", d.Span.Offset, d.Message, context)
		}
	}
}
