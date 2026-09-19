// A model whose names need quoting, as the OMG corpus writes them: a space, a
// keyword as a name, punctuation, and a quoted segment in the middle of a chain.
package 'My Pkg' {
	part def Car {
		attribute m = 5.0;
		attribute 'curb mass' = 1200.0;
	}

	calc 'add up' {
		in x;
		in y;
		x + y
	}

	constraint 'light enough' {
		5.0 < 10.0
	}

	requirement 'Safe Mass' {
		require constraint { 1200.0 < 2000.0 }
	}

	action 'run it' {
		attribute total = 0;

		first start;

		action accumulate {
			assign total := total + 5;
		}

		done;

		succession first start then accumulate;
		succession first accumulate then done;
	}

	state 'spin up' {
		entry; then init;
		state init;
		state waiting;
		accept after 10 [SI::s] then done;

		succession first init then waiting;
	}
}

// A satisfaction assertion whose element, requirement and subject all need
// quoting, so %satisfy is exercised on a name as well.
package 'Sat Pkg' {
	part def Lander {
		attribute 'vertical speed';
	}

	requirement def 'Touchdown Req' {
		subject lander : Lander;
		attribute 'max speed';
		require constraint {
			lander.'vertical speed' <= 'max speed'
		}
	}

	requirement 'touch down' : 'Touchdown Req' {
		attribute :>> 'max speed' = 1.5;
	}

	part 'slow lander' : Lander {
		attribute :>> 'vertical speed' = 1.2;
	}

	part 'the analysis' {
		assert satisfy 'touch down' by 'slow lander';
	}
}

package Top {
	package 'My Pkg' {
		part def Car {
			attribute m = 7.0;
		}
	}
}

// A keyword as a name, and a name with punctuation: neither holds a space, so
// both check that the quoting itself is what a command accepts.
package 'part' {
	part def Widget {
		attribute w = 2.0;
	}
}

package 'a-b.c' {
	part def Gadget {
		attribute g = 4.0;
	}
}
