// A collection function's body may be passed by reference: an `expr` parameter
// holding a body denotes that body, whether written with the arrow or by name.
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def ByArrow { in xs : Integer[*]; in expr pred; return : Integer[*] = xs->select pred; }
	calc def ByName { in xs : Integer[*]; in expr pred; return : Integer[*] = ControlFunctions::select(xs, pred); }
	calc def ByNamedArgs {
		in xs : Integer[*];
		in expr pred;
		return : Integer[*] = ControlFunctions::select(selector = pred, collection = xs);
	}

	calc def Positives {
		out byArrow : Integer[*] = ByArrow((-1, 2, 3), { in x; x > 0 });
		out byName : Integer[*] = ByName((-1, 2, 3), { in x; x > 0 });
		out byNamedArgs : Integer[*] = ByNamedArgs((-1, 2, 3), { in x; x > 0 });
	}

	calc p : Positives;
}
