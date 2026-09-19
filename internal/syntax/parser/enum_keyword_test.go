package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestEnumLiteralKeywordName(t *testing.T) {
	input := `
enum def StatusKind {
	done { doc /* done */ }
	closed;
}
`
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("%s", d.Message)
		}
	}
}
