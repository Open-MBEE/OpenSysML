// The definition-owned case runs on every object of the definition however
// deep the repeated part holding it sits: a bolt inside each of four wheels is
// four runs, the two front wheels' bolts torqued within the limit passing and
// the two rear ones failing.
package test {
	private import ScalarValues::Real;

	requirement def Tightness {
		subject b : Bolt;
		require constraint { b.torque < 10.0 }
	}
	requirement tight : Tightness;

	part def Bolt {
		attribute torque : Real default = 99.0;
		satisfy tight;

		verification torqueTest {
			objective torqueObjective { verify tight; }
			return verdict = VerificationCases::PassIf(tight(this));
		}
	}
	part def Wheel {
		part bolt : Bolt;
	}
	part def Car {
		part front : Wheel [2] {
			part :>> bolt {
				attribute :>> torque = 5.0;
			}
		}
		part rear : Wheel [2];
	}
	part car : Car;
}
