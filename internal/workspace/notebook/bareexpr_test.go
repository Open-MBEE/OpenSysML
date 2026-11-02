package notebook

import (
	"testing"
)

// TestBareExpression covers which inputs the kernel reads as expressions:
// only text the file grammar rejects and the expression grammar accepts whole.
func TestBareExpression(t *testing.T) {
	tests := []struct {
		input string
		expr  bool
	}{
		{input: "2 + 3", expr: true},
		{input: "1/0", expr: true},
		{input: "  sqrt(2.0)  ", expr: true},
		{input: "x", expr: true},
		{input: "part def Wheel;", expr: false},
		{input: "package P {\n}", expr: false},
		{input: "import ScalarValues::*;", expr: false},
		{input: "", expr: false},
		{input: "part def", expr: false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if _, got := BareExpression(tt.input); got != tt.expr {
				t.Errorf("BareExpression(%q) = %v, want %v", tt.input, got, tt.expr)
			}
		})
	}
}

// TestLoopEvaluatesBareExpressions checks the prompt answers an expression as
// %eval does, while declarations keep declaring.
