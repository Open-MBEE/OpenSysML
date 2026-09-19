// A body passed on to another calc keeps the environment it was written in:
// applied there, it still reads its writer's parameters and locals, even where
// the calc applying it declares a parameter of the same name.
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def Keep { in xs : Integer[*]; in expr pred; return : Integer[*] = xs->select pred; }
	calc def KeepShadowing {
		in xs : Integer[*];
		in expr pred;
		in threshold : Integer = 100;
		return : Integer[*] = xs->select pred;
	}
	calc def Sum { in xs : Integer[*]; in expr add; return : Integer = xs->reduce add; }

	calc def Above { in xs : Integer[*]; in threshold : Integer; return : Integer[*] = Keep(xs, { in x; x > threshold }); }
	calc def AboveShadowed {
		in xs : Integer[*];
		in threshold : Integer;
		return : Integer[*] = KeepShadowing(xs, { in x; x > threshold });
	}
	calc def AboveLocal {
		in xs : Integer[*];
		in threshold : Integer;
		attribute bound : Integer = threshold * 2;
		return : Integer[*] = Keep(xs, { in x; x > bound });
	}
	calc def WeightedSum {
		in xs : Integer[*];
		in weight : Integer;
		return : Integer = Sum(xs, { in acc; in x; acc + x * weight });
	}

	calc def Picks {
		out aboveTwo : Integer[*] = Above((1, 2, 3, 4), 2);
		out aboveTwoShadowed : Integer[*] = AboveShadowed((1, 2, 3, 4), 2);
		out aboveFour : Integer[*] = AboveLocal((1, 2, 3, 4, 5), 2);
		out weighted : Integer = WeightedSum((1, 2, 3), 10);
	}

	calc p : Picks;
}
