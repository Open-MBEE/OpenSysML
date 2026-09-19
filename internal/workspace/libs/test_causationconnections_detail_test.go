package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestCausationConnectionsDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Domain Libraries/Cause and Effect/CausationConnections.sysml")
	if err != nil {
		t.Fatalf("Failed to load CausationConnections.sysml: %v", err)
	}

	p := parser.New(source.New("CausationConnections.sysml", data))
	_ = p.ParseFile()

	t.Logf("CausationConnections.sysml: %d diagnostics", len(p.Diagnostics))

	for i, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(data) {
			char = string(data[byteOffset])
		}

		// Show 50 chars of context around error
		contextStart := byteOffset - 25
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := byteOffset + 25
		if contextEnd > len(data) {
			contextEnd = len(data)
		}
		context := string(data[contextStart:contextEnd])

		t.Logf("  %d. offset=%d (char=%q): %s", i+1, byteOffset, char, d.Message)
		t.Logf("     context: %q", context)
	}
}
