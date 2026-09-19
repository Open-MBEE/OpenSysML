package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestDebugParseIdKeyword(t *testing.T) {
	// Test if "do" excluded from identification
	code := `succession do`

	file := source.New("test.kerml", []byte(code))
	p := parser.New(file)
	_ = p.ParseFile()

	// If "do" excluded, no name should be parsed, error at "do" position
	if len(p.Diagnostics) == 0 {
		t.Fatal("Expected error, got none - means 'do' was accepted as name")
	}

	t.Logf("Diagnostics (expected):")
	for _, d := range p.Diagnostics {
		t.Logf("  offset %d: %s", d.Span.Offset, d.Message)
	}
}
