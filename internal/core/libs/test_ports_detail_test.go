package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestPortsDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Ports.sysml")
	if err != nil {
		t.Fatal(err)
	}

	p := parser.New(source.New("Ports.sysml", data))
	_ = p.ParseFile()

	t.Logf("Ports.sysml has %d diagnostics:", len(p.Diagnostics))
	for i, d := range p.Diagnostics {
		snippet := extractSnippet(string(data), d.Span.Offset, 50)
		t.Logf("  %d. [offset %d] %s", i+1, d.Span.Offset, d.Message)
		t.Logf("     Context: %q", snippet)
	}
}

func extractSnippet(content string, offset, length int) string {
	if offset < 0 || offset >= len(content) {
		return ""
	}
	end := offset + length
	if end > len(content) {
		end = len(content)
	}
	return content[offset:end]
}
