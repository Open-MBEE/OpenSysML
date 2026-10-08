package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestStatementNodeFanOut: two successions out of a statement node are two
// HappensBefore links, so both targets are performed once, whatever the node.
func TestStatementNodeFanOut(t *testing.T) {
	for _, tc := range []struct{ name, node string }{
		{"assign", "then assign x := 1;"},
		{"if", "then if x == 0 { assign x := 1; }"},
		{"while", "then while x < 1 { assign x := x + 1; }"},
		{"loop", "then loop { assign x := x + 1; } until x == 1;"},
		{"for", "then for i in (1, 2) { assign x := x + i; }"},
		{"send", "then send x to self;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `package test {
				private import ScalarValues::*;
				action fan {
					attribute x : Integer = 0;
					attribute b1 : Integer = 0;
					attribute c1 : Integer = 0;
					first start;
					` + tc.node + `
					then b;
					then c;
					action b { assign b1 := b1 + 1; }
					action c { assign c1 := c1 + 1; }
				}
			}`
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "fan", ast.DefAction)
			if sym == nil {
				t.Fatal("action fan not found")
			}
			results, err := ctx.ExecuteAction(sym)
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			if b, c := results["b1"].Const.Int, results["c1"].Const.Int; b != 1 || c != 1 {
				t.Errorf("b1 = %d, c1 = %d, want each target performed once", b, c)
			}
		})
	}
}
