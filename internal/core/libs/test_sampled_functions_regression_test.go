package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestSampledFunctionsRegression(t *testing.T) {
	src := &embedSource{}
	content, err := src.Read("Domain Libraries/Analysis/SampledFunctions.sysml")
	if err != nil {
		t.Fatal(err)
	}

	p := parser.New(source.New("SampledFunctions.sysml", content))
	_ = p.ParseFile()

	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  - Offset %d: %s", d.Span.Offset, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected clean parse, got %d errors", len(p.Diagnostics))
	}
}
