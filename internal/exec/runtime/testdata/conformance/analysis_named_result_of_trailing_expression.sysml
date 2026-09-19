// A usage naming the result its definition computes (`return mass;`) reads the
// value the definition's trailing result expression yields, under that name:
// the idiom of the pilot corpus 10a-Analysis, a case's result read by a nested
// sub-analysis. A calc usage names its result the same way.
package test {
	private import ScalarValues::*;

	part def Vehicle {
		attribute mass : Real = 200.0;
	}
	part vehicle : Vehicle;

	analysis def MassAnalysisCase {
		subject vehicle : Vehicle;
		vehicle.mass
	}

	calc def Doubled {
		in x : Real;
		x * 2.0
	}

	analysis plan {
		subject vehicle = test::vehicle;
		analysis massAnalysisCase : MassAnalysisCase {
			return mass;
		}
		calc doubled : Doubled {
			in x = 21.0;
			return twice;
		}
		out mass : Real = massAnalysisCase.mass;
		out twice : Real = doubled.twice;
	}
}
