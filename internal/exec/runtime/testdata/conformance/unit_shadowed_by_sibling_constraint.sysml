// A sibling named like an imported unit resolves in unit position by ordinary
// name resolution (KerML 8.2.3.5.3), and is not a unit: every path rejects it.
package test {
	public import SI::*;

	constraint def Tall {
		attribute m : ScalarValues::Real = 2.0;
		1.0 [m] > 500.0 [m]
	}
}
