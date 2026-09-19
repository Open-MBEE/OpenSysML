package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestDetailedErrors(t *testing.T) {
	src := &embedSource{}

	files := []string{
		"Domain Libraries/Analysis/StateSpaceRepresentation.sysml",
		"Systems Library/Cases.sysml",
		"Systems Library/VerificationCases.sysml",
	}

	for _, path := range files {
		data, err := src.Read(path)
		if err != nil {
			t.Errorf("Read %s: %v", path, err)
			continue
		}

		sf := source.New(path, data)
		p := parser.New(sf)
		_ = p.ParseFile()

		t.Logf("\n=== %s (%d errors) ===", path, len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 60 {
				text = text[:60] + "..."
			}
			t.Logf("  offset %d: %s [near: %q]", d.Span.Offset, d.Message, text)
		}
	}
}
