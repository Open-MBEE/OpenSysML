package test {
	private import ScalarValues::*;

	calc def Scale {
		in x : Integer;
		in factor : Integer = 2;
		out scaled = x * factor;
		out doubled = scaled + scaled;
	}

	// A definition redefining an inherited parameter (KerML 7.3.4.5), whose
	// outputs it inherits unchanged.
	calc def Tripled :> Scale {
		in redefines factor = 3;
	}

	// A usage binding the one input left unbound: the outputs it computes come
	// from parameters declared across all three calcs.
	calc t : Tripled {
		in x = 5;
	}
}
