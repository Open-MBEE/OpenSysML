package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestEnumLiteralWithBody(t *testing.T) {
	input := `
enum def StatusKind {
	open {
		doc /* Status is open */
	}
	tbd;
	closed = 2 {
		doc /* Status is closed */
	}
}
`
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("%s", d.Message)
		}
	}

	if root == nil {
		t.Fatal("Expected non-nil root")
	}
}
