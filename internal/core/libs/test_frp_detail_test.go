package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestFeatureReferencingPerformancesDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/FeatureReferencingPerformances.kerml")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	sf := source.New("FeatureReferencingPerformances.kerml", data)
	p := parser.New(sf)
	_ = p.ParseFile()

	t.Logf("FeatureReferencingPerformances.kerml: %d diagnostics", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}
		// Get context
		start := byteOffset - 30
		if start < 0 {
			start = 0
		}
		end := byteOffset + 30
		if end > len(data) {
			end = len(data)
		}
		context := string(data[start:end])

		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
		t.Logf("     Context: %q", context)
	}
}
