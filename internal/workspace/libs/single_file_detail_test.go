package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestSingleFile_RationalFunctions_Details(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Function Library/RationalFunctions.kerml")
	if err != nil {
		t.Fatal(err)
	}

	sf := source.New("RationalFunctions.kerml", data)
	p := parser.New(sf)
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			if i >= 10 {
				t.Logf("... and %d more", len(p.Diagnostics)-10)
				break
			}
			// Get text around error
			text := sf.Text(d.Span)
			if len(text) > 50 {
				text = text[:50] + "..."
			}
			t.Logf("  %d: %s | near: %q", i+1, d.Message, text)
		}
		t.Fatalf("Expected clean parse")
	}
}
