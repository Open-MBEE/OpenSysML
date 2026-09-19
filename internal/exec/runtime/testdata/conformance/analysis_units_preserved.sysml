// An analysis output computed over a quantity keeps its unit: doubling a mass in
// kilograms answers kilograms.
package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	part def Ship {
		attribute mass : MassValue = 10.0 [kg];
	}
	part ship : Ship;

	analysis unitful {
		subject s = ship;
		out doubled : MassValue = s.mass * 2.0;
	}
}
