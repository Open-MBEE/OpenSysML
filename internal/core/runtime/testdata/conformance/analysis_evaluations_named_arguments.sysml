// A case calc applied as a function value is one evaluation per distinct
// binding of its parameters: the same values bound by name to different
// parameters are two evaluations, reported by parameter position, while a
// value bound by position and then by name to the same parameter is one.
package test {
	private import ScalarValues::*;

	part def Ship { attribute cost : Real = 4.0; }
	part ship : Ship;

	analysis def Study {
		subject s : Ship;
		calc score {
			in a : Real;
			in plus : Real[0..1];
			in minus : Real[0..1];
			return : Real = a + (plus ?? 0.0) - (minus ?? 0.0);
		}
		in calc f = score;
		return : Real = f(a = 1.0, plus = 2.0) + f(a = 1.0, minus = 2.0) + f(1.0) + f(a = 1.0);
	}
	analysis study : Study { subject s = ship; }
}
