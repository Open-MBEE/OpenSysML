package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestCalcBodyStatingAFlowIsLoweredToAPerformance pins which calculation bodies
// lower to a token flow only a performance runs, and which keep statement order.
func TestCalcBodyStatingAFlowIsLoweredToAPerformance(t *testing.T) {
	tests := []struct {
		name, body string
		want       bool
	}{
		{"succession between action nodes", `action a; action b; first b then a; 1`, true},
		{"succession from start", `action a; first start then a; 1`, true},
		{"fork and join", `first start then f; fork f; then a; then b; action a; action b; join j; first a then j; first b then j; 1`, true},
		{"decision", `attribute x : Integer := 0; first start; then decide; if x > 0 then a; else done; action a; 1`, true},
		{"flow in a loop body", `attribute i : Integer := 1; while i > 0 { action a; action b { assign i := 0; } first b then a; } 1`, true},
		{"flow in a branch body", `if true { action a; action b; first b then a; } 1`, true},
		{"flow of a node in a loop body", `for i in 1..2 { action n { action a; action b; first b then a; } } 1`, true},
		{"ordered statements", `attribute y : Integer := 1; assign y := y * 2; then assign y := y + 1; y`, false},
		{"ordered statements in a loop body", `attribute i : Integer := 1; while i > 0 { assign i := i - 1; then assign i := i * 1; } i`, false},
		{"declaration-order node in a loop body", `attribute y : Integer := 1; for i in 1..2 { action a { assign y := y + i; } } y`, false},
		{"succession usage between action nodes", `action a; action b; succession first b then a; 1`, true},
		{"guarded succession between action nodes", `in n : Integer; action a; action b; first a if n > 0 then b; 1`, true},
		{"pins connected in a loop body", `for i in 1..2 { action a { out o : Integer = i; } action b { in p : Integer; } flow a.o to b.p; } 1`, true},
		{"succession naming no action node", `attribute x : Integer := 1; attribute y : Integer := 2; first x then y; 1`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := parser.New(source.New("calc.sysml", []byte("calc def C { "+tc.body+" }")))
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse: %v", p.Diagnostics)
			}
			def, ok := unwrapMembership(root.Members[0]).(*ast.Definition)
			if !ok {
				t.Fatal("no calc definition")
			}
			body, _ := CalcBodyWithOrder(def, def.Members, nil, nil, nil)
			if got := StatesFlow(CalcSteps(body)); got != tc.want {
				t.Fatalf("StatesFlow = %v, want %v", got, tc.want)
			}
		})
	}
}
