package passes

import "testing"

// bodyPrelude declares the collection functions the `->` notation names, so a
// body expression's operand resolves without the real library loaded.
const bodyPrelude = `package ControlFunctions {
	calc def collect { in source : ScalarValues::Integer[*]; in body; }
	calc def select { in source : ScalarValues::Integer[*]; in body; }
}
`

// A declaration inside an expression body is a member of the body's scope, so
// the type checker checks its bound value as it checks any other feature's, and
// reports it exactly once however often the body's type is inferred (F64).
func TestW7BBodyDeclarationValueIsTypeChecked(t *testing.T) {
	wantOneDiag(t, bodyPrelude+`package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	calc def C {
		in xs : Integer[*];
		attribute ys = xs->collect { in i; attribute k : Integer = "text"; k };
	}
}`, "cannot bind String value to a feature typed by Integer")
}

// The same inside a select predicate, whose body the checker types by its
// Boolean result rather than through the general body path.
func TestW7BSelectBodyDeclarationValueIsTypeChecked(t *testing.T) {
	wantOneDiag(t, bodyPrelude+`package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	calc def C {
		in xs : Integer[*];
		attribute ys = xs->select { in i; attribute k : Integer = "text"; i > k };
	}
}`, "cannot bind String value to a feature typed by Integer")
}

// A well-typed body declaration reports nothing, and a reference to it from the
// body's result is not a type error.
func TestW7BWellTypedBodyDeclarationIsClean(t *testing.T) {
	wantNoDiags(t, bodyPrelude+`package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	calc def C {
		in xs : Integer[*];
		attribute ys = xs->collect { in i; attribute k : Integer = i + 1; k };
	}
}`)
}
