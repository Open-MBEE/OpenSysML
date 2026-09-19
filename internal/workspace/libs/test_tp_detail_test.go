package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestTransitionPerformancesDetail(t *testing.T) {
	es := &embedSource{}
	data, err := es.Read("Kernel Libraries/Kernel Semantic Library/TransitionPerformances.kerml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	src := string(data)
	p := parser.New(source.New("TransitionPerformances.kerml", []byte(src)))
	_ = p.ParseFile()

	t.Logf("TransitionPerformances.kerml: %d diagnostics", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(src) {
			char = string(src[byteOffset])
		}

		// Get context around error
		start := byteOffset - 50
		if start < 0 {
			start = 0
		}
		end := byteOffset + 50
		if end > len(src) {
			end = len(src)
		}
		context := src[start:end]

		t.Logf("  - offset=%d (char=%q): %s", byteOffset, char, d.Message)
		t.Logf("    Context: %q", context)
	}
}
