package diagram_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestLayoutWithDeeplySharedDefinitions(t *testing.T) {
	var src strings.Builder
	src.WriteString("package Shared { private import Views::*; private import DiagramLayout::*; part def D0 { part leaf; }\n")
	for level := 1; level <= 12; level++ {
		fmt.Fprintf(&src, "part def D%d {\n", level)
		for branch := range 8 {
			fmt.Fprintf(&src, "part p%d : D%d;\n", branch, level-1)
		}
		src.WriteString("}\n")
	}
	src.WriteString("part root : D12; view v { expose root; render asTreeDiagram; metadata Layout about root { x = 0; y = 0; } } }")
	if diags := w8dDiags(t, src.String()); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}
