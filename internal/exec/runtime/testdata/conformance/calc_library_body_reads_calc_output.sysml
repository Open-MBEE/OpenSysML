// A body a calc yields keeps the outputs it names after that calc has returned,
// whether it is the calc's own bound result or read from a usage nested in its
// body: applied later, from a calc whose invocations have reused the storage the
// yielding calc ran in, its outputs are still worked out from the parameters it
// was invoked with.
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def Window {
		in n : Integer;
		out threshold : Integer = n;
		out limit : Integer = threshold * 10;
		out pred : expr = { in x; x > threshold and x < limit };
		bind result = pred;
	}
	calc def Band {
		in m : Integer;
		out threshold : Integer;
		out limit : Integer;
		out pred : expr;
	}
	calc def Nested {
		in n : Integer;
		in span : Integer;
		calc band : Band {
			in m = n;
			out threshold = m;
			out limit = threshold * span;
			out pred = { in x; x > threshold and x < limit };
		}
		return : expr = band.pred;
	}
	calc def Double { in n : Integer; return : Integer = n * 2; }
	calc def Keep { in xs : Integer[*]; in expr pred; return : Integer[*] = xs->select pred; }

	calc def Filter {
		in xs : Integer[*];
		in n : Integer;
		in expr within = Window(n);
		attribute churn : Integer = Double(n) + Double(1000);
		return : Integer[*] = Keep(xs, within);
	}
	calc def FilterNested {
		in xs : Integer[*];
		in n : Integer;
		in expr within = Nested(n, 10);
		attribute churn : Integer = Double(n) + Double(1000);
		return : Integer[*] = Keep(xs, within);
	}
	calc def FilterTwice {
		in xs : Integer[*];
		in expr low = Window(1);
		in expr high = Nested(3, 10);
		return : Integer[*] = Keep(Keep(xs, low), high);
	}

	calc def Picks {
		out aboveTwo : Integer[*] = Filter((1, 2, 3, 4, 25), 2);
		out aboveTwoNested : Integer[*] = FilterNested((1, 2, 3, 4, 25), 2);
		out aboveThree : Integer[*] = FilterTwice((1, 2, 3, 4, 5, 40));
	}

	calc p : Picks;
}
