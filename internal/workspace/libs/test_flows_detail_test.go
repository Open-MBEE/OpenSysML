package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestFlowsDetail(t *testing.T) {
	embedSrc := &embedSource{}
	data, err := embedSrc.Read("Systems Library/Flows.sysml")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	src := source.New("Flows.sysml", data)
	p := parser.New(src)
	_ = p.ParseFile()

	t.Logf("Flows.sysml: %d diagnostics", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}
		context := ""
		if byteOffset >= 20 && byteOffset+20 < len(data) {
			context = string(data[byteOffset-20 : byteOffset+20])
		}
		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
		t.Logf("     context: %q", context)
	}
}
