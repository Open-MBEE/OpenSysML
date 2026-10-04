package test {
	private import ScalarValues::*;

	// `===` over sequences is element-wise identity, as SequenceFunctions::same is:
	// an Integer is never the same as an equal Real, at any position or depth.
	calc def Mixed { in a : Real; return : Boolean = (1, a) === (1.0, a); }
	calc def Kept { in a : Real; return : Boolean = (1, a) === (1, a); }
	calc def Differ { in a : Real; return : Boolean = (1, a) !== (1.0, a); }
	calc def Nested { in a : Real; return : Boolean = ((1, 2), a) === ((1, 2.0), a); }
	calc def Equal { in a : Real; return : Boolean = (1, a) == (1.0, a); }

	action def Identities {
		out mixed : Boolean = Mixed(2.5);
		out kept : Boolean = Kept(2.5);
		out differ : Boolean = Differ(2.5);
		out nested : Boolean = Nested(2.5);
		out equal : Boolean = Equal(2.5);
	}
	action identities : Identities;
}
