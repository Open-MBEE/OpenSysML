// A part whose attributes state values written three ways — bound with `=`,
// given as a `default`, set as a `:=` initial value — on the part itself,
// inherited from its definition and supplied by a redefinition, with a derived
// one over them, so a query can tell what the model binds from what it leaves open.
package test {
	private import ScalarValues::*;

	part def Meter {
		attribute inherited : Real = 3.0;
		attribute fallback : Real default 4.0;
		attribute rebound : Real default 5.0;
	}

	part Gauge : Meter {
		attribute bound : Real = 1.0;
		attribute :>> fallback;
		attribute :>> rebound = 5.0;
		attribute initial : Real := 2.0;
		attribute doubled : Real = bound * 2.0;

		assert constraint inRange {
			bound + inherited + rebound + fallback + initial + doubled <= 100.0
		}
	}
}
