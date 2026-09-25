// A redefinition stating no multiplicity keeps the inherited `[0..1]`, so the
// specialization is still called without that argument.
package test {
	private import ScalarValues::*;

	calc def Scale {
		in x : Integer;
		in factor : Integer[0..1];
		return : Integer = x * (factor ?? 1);
	}

	calc def Doubling :> Scale {
		in redefines x;
		in redefines factor;
		return : Integer = x * (factor ?? 2);
	}

	calc def Scaling {
		out omitted = Doubling(5);
		out given = Doubling(5, 3);
	}

	calc c : Scaling;
}
