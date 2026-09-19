package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestOccurrencesSubsetStatements(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/Occurrences.kerml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	p := parser.New(source.New("Occurrences.kerml", data))
	_ = p.ParseFile()

	t.Logf("Diagnostics count: %d", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		if i < 20 { // Show first 20
			t.Logf("  %s at offset %d", d.Message, d.Span.Offset)
		}
	}
}
