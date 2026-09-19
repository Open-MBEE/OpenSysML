package test {
	private import ScalarValues::*;

	// One output and no `return`: that output is the calculation's result, so
	// the calc stays invocable as an expression while the usage's output is
	// readable as a feature.
	calc def Increment {
		in n : Integer;
		out a = n + 1;
	}

	calc c : Increment {
		in n = 5;
	}

	attribute viaFeature = c.a;
	attribute viaInvocation = Increment(5);
}
