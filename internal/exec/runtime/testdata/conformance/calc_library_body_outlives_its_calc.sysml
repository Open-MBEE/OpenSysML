// A body a calc returns keeps the parameter it closes over after that calc has
// returned: applied later, from another calc whose invocations have reused the
// storage the returning calc ran in, it still reads the value it was made with.
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def AboveThreshold { in threshold : Integer; return : expr = { in x; x > threshold }; }
	calc def Double { in n : Integer; return : Integer = n * 2; }
	calc def Keep { in xs : Integer[*]; in expr pred; return : Integer[*] = xs->select pred; }

	calc def Filter {
		in xs : Integer[*];
		in threshold : Integer;
		in expr above = AboveThreshold(threshold);
		attribute churn : Integer = Double(threshold) + Double(1000);
		return : Integer[*] = Keep(xs, above);
	}
	calc def FilterTwice {
		in xs : Integer[*];
		in expr low = AboveThreshold(1);
		in expr high = AboveThreshold(3);
		return : Integer[*] = Keep(Keep(xs, low), high);
	}

	calc def Picks {
		out aboveTwo : Integer[*] = Filter((1, 2, 3, 4), 2);
		out aboveThree : Integer[*] = FilterTwice((1, 2, 3, 4, 5));
	}

	calc p : Picks;
}
