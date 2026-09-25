package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestTradeStudiesDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Domain Libraries/Analysis/TradeStudies.sysml")
	if err != nil {
		t.Fatalf("Failed to load TradeStudies.sysml: %v", err)
	}

	sf := source.New("Domain Libraries/Analysis/TradeStudies.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()

	t.Logf("TradeStudies.sysml: %d diagnostics", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}

		// Get context (20 chars before/after)
		start := byteOffset - 20
		if start < 0 {
			start = 0
		}
		end := byteOffset + 20
		if end > len(data) {
			end = len(data)
		}
		context := string(data[start:end])

		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
		t.Logf("      context: %q", context)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
