// A sibling named like an imported unit resolves in unit position by ordinary
// name resolution (KerML 8.2.3.5.3), and is not a unit: every path rejects it.
package test {
	public import SI::*;

	part def Probe {
		attribute m : ISQBase::MassValue = 1000.0 [kg];
		attribute h : ISQBase::LengthValue = 500.0 [m];
	}
}
