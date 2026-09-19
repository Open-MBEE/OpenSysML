package test {
	private import ScalarValues::*;

	calc def Split {
		in n : Integer;
		out low = n - 1;
		out high = n + 1;
	}

	// The nested usage's input has the name of the parameter it binds from, which
	// shadows it the way a nested binding does elsewhere: `in n = n` reads the
	// enclosing `n` rather than depending on itself.
	calc def Span {
		in n : Integer;
		calc s : Split {
			in n = n;
		}
		out width = s.high - s.low;
		out top = s.high;
	}

	calc spanOfFive : Span {
		in n = 5;
	}

	// The same nested usage read from another invocation, whose inputs differ, so
	// it runs again rather than answering with the first invocation's values.
	calc spanOfTen : Span {
		in n = 10;
	}

	attribute tenTop = spanOfTen.top;
	attribute tenWidth = spanOfTen.width;

	calc def Pair {
		in n : Integer;
		in m : Integer;
		out d = n * 10 + m;
	}

	// Every input binds in the enclosing environment, so the two values swap:
	// `in m = n` reads the enclosing `n`, not the `n` bound just above it.
	calc def Swap {
		in n : Integer;
		in m : Integer;
		calc p : Pair {
			in n = m;
			in m = n;
		}
		out d = p.d;
	}

	calc swapped : Swap {
		in n = 1;
		in m = 2;
	}

	attribute swappedD = swapped.d;

	calc def Doubler {
		in a : Integer;
		in b : Integer = a * 2;
		out d = b;
	}

	// An input redeclared without a value keeps the default its definition
	// states, evaluated there: `b` is twice the usage's `a`, not the enclosing one.
	calc def Held {
		in a : Integer;
		calc u : Doubler {
			in a = 1;
			in :>> b;
		}
		out d = u.d;
	}

	calc held : Held {
		in a = 9;
	}

	attribute heldD = held.d;
}
