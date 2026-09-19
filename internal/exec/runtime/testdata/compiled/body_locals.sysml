// Calc bodies of statements: scalar locals in declaration order, locals that
// mask parameters and outer locals, conditionals whose branches return, and the
// forms that keep the evaluator — a local read before its declaration, a local
// without a value, a body that may run off its end.
package test {
	private import ScalarValues::*;

	// A later local reads an earlier one; the return reads both.
	calc def Chain {
		in x : Real;
		attribute a : Real = x * 2.0;
		attribute b = a + 1.0;
		return : Real = a * b;
	}

	// A local named like the parameter masks it for the statements after it.
	calc def MasksParameter {
		in k : Integer;
		attribute k2 = k + 1;
		attribute k = k2 * 2;
		return : Integer = k + k2;
	}

	// Each branch declares a local of its own and returns it.
	calc def Branches {
		in x : Real;
		if x < 0.0 {
			attribute y = -x;
			return : Real = y * 2.0;
		} else {
			attribute y = x;
			return : Real = y;
		}
	}

	// A conditional without an else, then the return every other path reaches.
	calc def Clamp {
		in k : Integer;
		if k > 10 {
			return : Integer = 10;
		}
		if k < -10 {
			return : Integer = -10;
		}
		return : Integer = k;
	}

	// A block local masks an outer local of the same name inside the block only.
	calc def Shadow {
		in k : Integer;
		attribute a = k;
		if k > 0 {
			attribute a = k * 10;
			return : Integer = a;
		}
		return : Integer = a;
	}

	// Nested conditionals, each branch of which returns.
	calc def Sign {
		in x : Real;
		if x < 0.0 {
			return : Integer = -1;
		} else {
			if x > 0.0 {
				return : Integer = 1;
			} else {
				return : Integer = 0;
			}
		}
	}

	// The local's initializer may fail: the failure is worded as a declaration's.
	calc def FailsInLocal {
		in k : Integer;
		attribute a : Integer = 100 % k;
		return : Integer = a;
	}

	// The returned value answers to the result's declaration.
	calc def NaturalResult {
		in k : Integer;
		attribute a = k - 1;
		return : Natural = a;
	}

	// The condition must be a Boolean; a Real passed for the untyped parameter is
	// refused as the body refuses it.
	calc def BadCondition {
		in x;
		if x {
			return : Real = 1.0;
		}
		return : Real = 0.0;
	}

	// Locals compute with locals of the callee's own frame across a call.
	calc def CallsChain {
		in x : Real;
		attribute a = Chain(x);
		attribute b = Chain(a);
		return : Real = a + b;
	}

	// Evaluator only: a local read before its declaration.
	calc def ReadsEarly {
		in k : Integer;
		attribute a : Integer = b + k;
		attribute b : Integer = 1;
		return : Integer = a;
	}

	// Evaluator only: a local declared without a value.
	calc def NoValue {
		in k : Integer;
		attribute a : Integer;
		return : Integer = k;
	}

	// Evaluator only: a body that may run off its end.
	calc def MayFall {
		in k : Integer;
		if k > 0 {
			return : Integer = k;
		}
	}

	// Evaluator only: a local holding a String.
	calc def StringLocal {
		in k : Integer;
		attribute s : String = "x";
		return : Integer = k;
	}

	// Evaluator only: an assignment to a local.
	calc def Assigns {
		in k : Integer;
		attribute a : Integer = k;
		assign a := a + 1;
		return : Integer = a;
	}
}
