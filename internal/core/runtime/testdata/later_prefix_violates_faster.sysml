package test {
	private import ScalarValues::*;

	// A violation a later, wider subtree reaches faster than the first prefix: the
	// decision's first branch computes at length and then divides by the zero `x`,
	// while its second branch forks three writers of `x` before the same division,
	// so the second subtree is six linearizations wide to the first's one and two
	// of its runs reach the same failure in a few steps. Not a conformance case: the
	// harness admits no erroring outcome, and this one is the point.
	calc fib {
		in n : Integer;
		return : Integer = if n < 2 ? n else fib(n - 1) + fib(n - 2);
	}

	action race {
		attribute x : Integer = 0;
		attribute y : Integer = 0;
		attribute w : Integer = 0;

		first start;
		decide pick;
		action slow { assign w := fib(12); }
		action quick { assign w := 1; }
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		action c { assign x := 0; }
		join sync;
		merge meet;
		action divide { assign y := 1 / x; }
		done;

		succession first start then pick;
		succession first pick if w == 0 then slow;
		succession first pick if w == 0 then quick;
		succession first slow then meet;
		succession first quick then split;
		succession first split then a;
		succession first split then b;
		succession first split then c;
		succession first a then sync;
		succession first b then sync;
		succession first c then sync;
		succession first sync then meet;
		succession first meet then divide;
		succession first divide then done;
	}
}
