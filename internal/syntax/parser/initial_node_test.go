package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestFirstStartThenOff(t *testing.T) {
	src := `package test {
	state s {
		first start then off;
		state off;
	}
}`

	p := New(source.New("test", []byte(src)))
	_ = p.ParseFile()

	t.Logf("Parse diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		t.Logf("  %v", d)
	}

	// Just check no parse errors
	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected 0 parse diagnostics, got %d", len(p.Diagnostics))
	}
}
