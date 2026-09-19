// Invocations binding their arguments by name: in declaration order, reordered,
// leaving a defaulted parameter to its default, and against a library function.
package test {
	private import ScalarValues::*;
	private import RealFunctions::*;

	calc def Weighted {
		in value : Real;
		in weight : Real = 1.0;
		in offset : Real = 0.0;
		return : Real = value * weight + offset;
	}

	calc def InOrder { in v : Real; return : Real = Weighted(value = v, weight = 2.0, offset = 1.0); }
	calc def Reordered { in v : Real; return : Real = Weighted(offset = 1.0, weight = 2.0, value = v); }
	calc def Defaulted { in v : Real; return : Real = Weighted(weight = 3.0, value = v); }
	calc def OnlyRequired { in v : Real; return : Real = Weighted(value = v); }

	// The argument expressions are evaluated in source order, so the first to
	// fail is the one reported.
	calc def Failing { in k : Integer; return : Real = Weighted(offset = 1.0 / k, value = k % 0); }

	// A library function's parameters are bound by name too.
	calc def LibraryNamed { in a : Real; in b : Real; return : Real = max(y = b, x = a); }

	// A recursion binding its argument by name.
	calc def Fib { in k : Integer; return : Integer = if k <= 1 ? k else Fib(k = k - 1) + Fib(k = k - 2); }

	// A statement body whose locals feed named arguments.
	calc def Scaled {
		in v : Real;
		attribute w = v + 1.0;
		attribute r = Weighted(weight = w, value = v);
		return : Real = r;
	}
}
