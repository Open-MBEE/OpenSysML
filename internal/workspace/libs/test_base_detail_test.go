package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestBaseDetail(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Semantic Library/Base.kerml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	p := parser.New(source.New("Base.kerml", data))
	_ = p.ParseFile()

	t.Logf("Total diagnostics: %d\n", len(p.Diagnostics))

	for i, d := range p.Diagnostics {
		t.Logf("\nDiagnostic %d:", i+1)
		t.Logf("  Offset: %d", d.Span.Offset)
		t.Logf("  Message: %s", d.Message)

		// Context
		offset := d.Span.Offset
		start := offset - 40
		if start < 0 {
			start = 0
		}
		end := offset + 40
		if end > len(data) {
			end = len(data)
		}
		ctx := string(data[start:end])
		t.Logf("  Context: ...%q...", ctx)
	}
}
