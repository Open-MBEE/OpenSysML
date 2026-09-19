package model

import (
	"testing"
)

func TestSimpleMetaclass(t *testing.T) {
	ws := NewWorkspace()

	src := `package Test {
		metaclass MyMetaclass;
	}`

	ws.Open("test.sysml", []byte(src), 1)

	symbols := ws.index.LookupQualified("Test::MyMetaclass")
	t.Logf("Found %d symbols", len(symbols))
	for i, sym := range symbols {
		t.Logf("  Symbol %d: kind=%s", i, sym.Kind)
	}
}
