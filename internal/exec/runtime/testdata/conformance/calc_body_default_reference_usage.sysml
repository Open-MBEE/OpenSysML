package test {
	private import ScalarValues::*;

	// A kind-less `name = expr;` or `name := expr;` in a calculation body declares
	// a feature of the calculation (SysML.xtext DefaultReferenceUsage), not an
	// assignment; the trailing result expression reads such locals by name.
	calc def Affine {
		in n : Integer;
		twice = n * 2;
		offset := 1;
		twice + offset
	}
}
