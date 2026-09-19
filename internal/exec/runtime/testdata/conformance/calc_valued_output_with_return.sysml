// A calc whose body computes a result keeps that value to itself: an output
// states its own binding, and reading it answers that binding rather than the
// result expression, even when the invocation hands that output back.
package test {
	private import ScalarValues::*;

	// The result expression and the output compute different things.
	calc def Apart {
		in n : Integer;
		out a : Integer = n + 1;
		n * 100
	}

	// The return names the output, so the two agree.
	calc def Together {
		in n : Integer;
		out r : Integer = n * 2;
		return r;
	}

	// Two valued outputs designate no result, so the result expression is the invocation's.
	calc def Both {
		in n : Integer;
		out a : Integer = n + 1;
		out b : Integer = n + 2;
		n * 100
	}

	part h {
		calc ca : Apart { in n = 5; }
		calc ct : Together { in n = 5; }
		calc cb : Both { in n = 5; }

		attribute apartOutput : Integer = ca.a;
		attribute apartValue : Integer = Apart(5);
		attribute togetherOutput : Integer = ct.r;
		attribute togetherValue : Integer = Together(5);
		attribute bothFirst : Integer = cb.a;
		attribute bothSecond : Integer = cb.b;
		attribute bothValue : Integer = Both(5);
	}
}
