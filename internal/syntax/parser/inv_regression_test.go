package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestInvAfterEnumBodySupport(t *testing.T) {
	input := `
function f {
	inv { isZero(zero) }
}
`

	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no errors, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s", d.Message)
		}
		t.Fail()
	}

	if root == nil {
		t.Fatal("ParseFile returned nil")
	}
}
