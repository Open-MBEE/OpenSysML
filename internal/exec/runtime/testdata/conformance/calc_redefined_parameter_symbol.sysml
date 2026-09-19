package test {
	// A parameter may be written as a redefinition of the one it overrides
	// (`in :>> factor = 3;`), as the shipped Kernel libraries do
	// (ControlPerformances.kerml `in redefines ifTest;`). Such a declaration
	// states no name of its own and answers to the name of what it redefines
	// (KerML 7.3.4.5), so it must still be a parameter of the invocation. The
	// result expression is inherited: a specialization may not state a second.
	calc def Scaled :> lib::Scale {
		in :>> factor = 3;
	}

	package lib {
		calc def Scale {
			in x : Integer;
			in factor : Integer = 2;
			x * factor
		}
	}
}
