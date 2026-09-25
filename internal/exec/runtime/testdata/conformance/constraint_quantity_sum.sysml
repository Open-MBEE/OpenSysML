// A constraint summing quantities of the same dimension in different units:
// 3 [km] + 500 [m] is 3.5 [km], which is 3500 [m].
package test {
	public import SI::*;

	constraint quantitiesAdd {
		3.0 [km] + 500.0 [m] <= 3500.0 [m]
	}
}
