// Parameters resolved along the specialization chain: inherited, redeclared in
// place, given a default or a narrower type by a redeclaration, three levels deep.
package test {
	private import ScalarValues::*;

	calc def Base {
		in x : Real;
		in y : Real = 1.0;
		return : Real = x * y;
	}

	// Level two redeclares y in place with another default.
	calc def Mid :> Base {
		in :>> y = 2.0;
	}

	// Level three redeclares x with a default and inherits Mid's y.
	calc def Top :> Mid {
		in :>> x = 3.0;
	}

	// A redeclaration naming the base's parameter by a qualified name.
	calc def Qualified :> Base {
		in x :>> Base::x = 10.0;
	}

	// A redeclaration narrowing Real to Integer: a Real argument is refused as
	// the parameter's declaration refuses it, an Integer computes as a Real would.
	calc def Narrow :> Base {
		in :>> x : Integer;
	}

	// A specialization with its own body reads the inherited parameters.
	calc def Own :> Base {
		return : Real = x - y;
	}

	// A subtype adds a parameter after the inherited ones.
	calc def Extended :> Base {
		in z : Real = 0.5;
		return : Real = x * y + z;
	}

	// A body of statements over inherited and redeclared parameters.
	calc def Statements :> Mid {
		attribute a = x + y;
		return : Real = a * a;
	}

	// The inherited body is invoked through the specialization.
	calc def CallsTop {
		in k : Real;
		return : Real = Top(k) + Top() + Narrow(2);
	}

	// Evaluator only: the redeclaration binds a non-literal default.
	calc def ComputedDefault :> Base {
		in :>> y = x + 1.0;
	}
}
