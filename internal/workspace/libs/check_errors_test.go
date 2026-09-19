package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestCheckFileErrors(t *testing.T) {
	files := []string{
		"Domain Libraries/Metadata/RiskMetadata.sysml",
		"Domain Libraries/Quantities and Units/ISQSpaceTime.sysml",
		"Kernel Libraries/Kernel Function Library/SequenceFunctions.kerml",
		"Kernel Libraries/Kernel Semantic Library/Links.kerml",
	}

	src := &embedSource{}
	for _, name := range files {
		data, err := src.Read(name)
		if err != nil {
			t.Fatal(err)
		}

		sf := source.New(name, data)
		p := parser.New(sf)
		_ = p.ParseFile()

		t.Logf("\n%s: %d errors", name, len(p.Diagnostics))
		if len(p.Diagnostics) > 0 && len(p.Diagnostics) <= 10 {
			for _, d := range p.Diagnostics {
				text := sf.Text(d.Span)
				if len(text) > 30 {
					text = text[:30] + "..."
				}
				t.Logf("  %s [near: %q]", d.Message, text)
			}
		}
	}
}
