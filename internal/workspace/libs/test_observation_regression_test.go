package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestObservationRegression(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/Observation.kerml")
	if err != nil {
		t.Fatal(err)
	}

	p := parser.New(source.New("Observation.kerml", data))
	_ = p.ParseFile()

	t.Logf("Observation.kerml: %d diagnostics", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}
		// Get context
		start := byteOffset - 80
		if start < 0 {
			start = 0
		}
		end := byteOffset + 80
		if end > len(data) {
			end = len(data)
		}
		context := string(data[start:end])

		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
		t.Logf("     Context: %q", context)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("REGRESSION: Observation.kerml was clean, now has %d errors", len(p.Diagnostics))
	}
}
