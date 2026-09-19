package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestRefUsageInActionBodyParsesClean covers a bare `ref` usage declared in an
// action body, which no golden fixture declares.
func TestRefUsageInActionBodyParsesClean(t *testing.T) {
	input := `action def Test {
ref stateSpace: StateSpace;
}`
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	_ = p.ParseFile()

	for _, d := range p.Diagnostics {
		t.Errorf("parse error: %s", d.Message)
	}
}
