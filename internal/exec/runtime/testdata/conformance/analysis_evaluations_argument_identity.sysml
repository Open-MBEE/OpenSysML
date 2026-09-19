// Evaluations are distinct by the values they bind, not by how the trace spells
// them: one calc read off two parts is two functions, so applying the case's
// calc to each is two evaluations; a whole Real and the Integer it equals are
// one value, so applying it to each is one.
package test {
	private import ScalarValues::*;

	part def Scaler {
		attribute k : Real;
		calc scale { in x : Real; return : Real = x * k; }
	}
	part twice : Scaler { attribute :>> k = 2.0; }
	part thrice : Scaler { attribute :>> k = 3.0; }

	analysis def Study {
		subject s : Scaler;
		calc apply {
			in calc g { in x : Real; return : Real; }
			in a : Real;
			return : Real = g(a);
		}
		in calc f = apply;
		return : Real = f(twice.scale, 5.0) + f(thrice.scale, 5.0) + f(s.scale, 1) + f(s.scale, 1.0);
	}
	analysis study : Study { subject s = twice; }
}
