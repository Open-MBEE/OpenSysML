// A constraint over a quantity whose unit is composed by division:
// 10 [m] / 2 [s] is 5 [m/s], which is more than 4 [km/h].
package test {
	public import SI::*;

	constraint quantityDivides {
		10.0 [m] / 2.0 [s] > 4.0 [km/h]
	}
}
