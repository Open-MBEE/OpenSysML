package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): three branches each
	// write `x` once between one fork and one join, the first computing its value
	// by a recursion that takes many evaluations while the other two write a
	// literal; the library orders none of them against another, so the six orders
	// of the three writes leave three values open — the last write stands.
	calc fib {
		in n : Integer;
		return : Integer = if n < 2 ? n else fib(n - 1) + fib(n - 2);
	}

	action race {
		attribute x : Integer = 0;

		first start;
		fork split;
		action slow { assign x := fib(12); }
		action b { assign x := 2; }
		action c { assign x := 3; }
		join sync;
		done;

		succession first start then split;
		succession first split then slow;
		succession first split then b;
		succession first split then c;
		succession first slow then sync;
		succession first b then sync;
		succession first c then sync;
		succession first sync then done;
	}
}
