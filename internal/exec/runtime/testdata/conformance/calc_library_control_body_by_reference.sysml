// A Control function's `expr` operand may be passed on by name: the branch it
// selects is applied where the calc naming it received a body, and it is
// applied in the environment that body was written in.
package test {
	private import ScalarValues::*;

	calc def Pick {
		in test : Boolean;
		in expr whenTrue;
		in expr whenFalse;
		return : Integer = ControlFunctions::'if'(test, whenTrue, whenFalse);
	}
	calc def Fallback { in primary : Integer[0..1]; in expr otherwise; return : Integer = ControlFunctions::'??'(primary, otherwise); }
	calc def Both { in primary : Boolean; in expr second; return : Boolean = ControlFunctions::'and'(primary, second); }

	calc def Offset { in base : Integer; return : Integer = Pick(base > 0, { base + 1 }, { 0 - base }); }
	calc def Doubled { in value : Integer; return : Integer = Fallback(null, { value * 2 }); }
	calc def Within { in value : Integer; in limit : Integer; return : Boolean = Both(value > 0, { value < limit }); }

	calc def Picks {
		out positiveOffset : Integer = Offset(4);
		out negativeOffset : Integer = Offset(-4);
		out doubled : Integer = Doubled(21);
		out within : Boolean = Within(3, 5);
		out beyond : Boolean = Within(7, 5);
	}

	calc p : Picks;
}
