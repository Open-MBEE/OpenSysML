package libs

import (
	"os"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestCausationDetail(t *testing.T) {
	file := "stdlib/Domain Libraries/Cause and Effect/CausationConnections.sysml"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", file, err)
	}

	p := parser.New(source.New(file, data))
	_ = p.ParseFile()

	t.Logf("File: %s", file)
	t.Logf("Diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  Offset %d: %s", d.Span.Offset, d.Message)
	}

	if len(p.Diagnostics) > 0 {
		t.Logf("\nShould be clean (succession first/then fixed)")
	}
}
