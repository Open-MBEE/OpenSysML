// A unit position naming nothing at all still fails as an unresolved name,
// which is a different diagnostic than a name that resolves to a non-unit.
package test {
	public import SI::*;

	calc def Height {
		return : ISQBase::LengthValue = 500.0 [furlong];
	}
}
