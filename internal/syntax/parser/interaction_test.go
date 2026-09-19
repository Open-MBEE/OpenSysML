package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestInteractionKeyword(t *testing.T) {
	input := `
package Test {
	interaction Transfer specializes BinaryLink {
		doc /* Transfer represents payload transfer */
		end feature source: Occurrence;
	}
}
`
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no diagnostics, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}
