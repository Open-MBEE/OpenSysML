package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestSingleFile_RationalFunctions(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Kernel Libraries/Kernel Function Library/RationalFunctions.kerml")
	if err != nil {
		t.Fatal(err)
	}

	p := parser.New(source.New("RationalFunctions.kerml", data))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			if i >= 10 {
				t.Logf("... and %d more", len(p.Diagnostics)-10)
				break
			}
			t.Logf("  %d: %s", i+1, d.Message)
		}
		t.Fatalf("Expected clean parse")
	}
}
