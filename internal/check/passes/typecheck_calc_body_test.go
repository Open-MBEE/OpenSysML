package passes

import "testing"

// TestCalcBodyIterativeIsClean: a calculation body that declares locals, loops
// and returns types clean, as the same statements do in an action.
func TestCalcBodyIterativeIsClean(t *testing.T) {
	wantNoDiags(t, `package P {
		calc def Factorial {
			in n : ScalarValues::Integer;
			attribute acc : ScalarValues::Integer = 1;
			attribute i : ScalarValues::Integer = 1;
			while i <= n {
				acc = acc * i;
				i = i + 1;
			}
			return : ScalarValues::Integer = acc;
		}
	}`)
}

// TestCalcBodyNonBooleanWhileCondition: a loop condition in a calc must be
// Boolean, as it must be in an action.
func TestCalcBodyNonBooleanWhileCondition(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def C {
			in n : ScalarValues::Integer;
			while n {
				return : ScalarValues::Integer = n;
			}
			return : ScalarValues::Integer = 0;
		}
	}`, "condition of 'while' must be Boolean")
}

// TestCalcBodyNonBooleanIfCondition: same rule for a conditional's condition.
func TestCalcBodyNonBooleanIfCondition(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def C {
			in n : ScalarValues::Integer;
			if n {
				return : ScalarValues::Integer = 1;
			}
			return : ScalarValues::Integer = 0;
		}
	}`, "condition of 'if' must be Boolean")
}

// TestCalcBodyReturnInsideBranchIsChecked: a `return` nested in a branch is
// typed in that branch's scope, so an ill-typed returned expression is reported
// wherever the return is written.
func TestCalcBodyReturnInsideBranchIsChecked(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def C {
			in n : ScalarValues::Integer;
			if n > 0 {
				return : ScalarValues::Integer = n + "one";
			}
			return : ScalarValues::Integer = 0;
		}
	}`, "operator '+' is not defined for Integer and String")
}

// TestCalcBodyReturnInsideLoopIsChecked: and the same for a `return` in a loop
// body, whose locals it may read.
func TestCalcBodyReturnInsideLoopIsChecked(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def C {
			in xs : ScalarValues::Integer[*];
			for x in xs {
				attribute flag : ScalarValues::Boolean = true;
				return : ScalarValues::Integer = flag * 2;
			}
			return : ScalarValues::Integer = 0;
		}
	}`, "operator '*' requires numeric operands, found Boolean and Natural")
}

// TestCalcBodyLoopLocalIsNotVisibleOutside: a name a loop body declares is a
// member of that body, so reading it after the loop does not resolve.
func TestCalcBodyLoopLocalIsNotVisibleOutside(t *testing.T) {
	src := `package P {
		calc def C {
			in n : ScalarValues::Integer;
			while n > 0 {
				attribute inner : ScalarValues::Integer = n;
			}
			return : ScalarValues::Integer = inner;
		}
	}`
	if diags := exprDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected the unresolved name to be reported by name resolution, got type diagnostics %v", diags)
	}
}
