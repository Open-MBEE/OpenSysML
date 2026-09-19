// A calculation whose result is a ratio of like quantities, which is a number
// of no unit: 4 [m] / 2 [m] is 2, and 1 [km] / 1 [m] is 1000.
package test {
	public import SI::*;

	calc def Ratio {
		return : ScalarValues::Real = (4.0 [m] / 2.0 [m]) + (1.0 [km] / 1.0 [m]);
	}
}
