package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestStepSimple(t *testing.T) {
	input := `
package Test {
	action A {
		step entry[1];
		step do[1] subsets middle;
		step exit[1];
	}
}
`
	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no diagnostics, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s at offset %d", d.Message, d.Span.Offset)
		}
	}
}

func TestStepStatePerformances(t *testing.T) {
	input := `
package Test {
	action StatePerformance {
		step entry[1];
		step do[1] subsets middle;
		step exit[1];
		
		step nonDoMiddle[*] subsets middle;
	}
}
`
	p := parser.New(source.New("test.kerml", []byte(input)))
	_ = p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no diagnostics, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s at offset %d", d.Message, d.Span.Offset)
		}
	}
}
