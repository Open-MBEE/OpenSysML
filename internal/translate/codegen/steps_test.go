package codegen

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A Let whose value cannot fail spends its body's leading steps with its own;
// one whose value can fail keeps them after the value, where a failure precedes them.
func TestChargedMergesStepsAcrossAnInfallibleLet(t *testing.T) {
	v := Var{Name: "v", T: TypeReal}
	product := Binary{Op: ast.OpMul, L: v, R: v, T: TypeReal}
	body := Steps{N: 2, X: product}

	got := charged(1, Let{Name: "t", Value: Steps{N: 3, X: v}, In: body})
	want := Steps{N: 6, X: Let{Name: "t", Value: v, In: product}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("infallible value: got %#v, want %#v", got, want)
	}

	got = charged(1, Let{Name: "t", Value: Steps{N: 3, X: product}, In: body})
	want = Steps{N: 4, X: Let{Name: "t", Value: product, In: body}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fallible value: got %#v, want %#v", got, want)
	}
}
