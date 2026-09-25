// A composite part-typed collection given a default holds the objects the
// default names rather than anonymous objects of its type, and an expression
// default over other features holds what the expression produced.
package test {
	private import ScalarValues::Real;

	part def Sub {
		attribute volume : Real;
	}

	part def Bay {
		part left : Sub {
			attribute :>> volume = 2.0;
		}
		part right : Sub {
			attribute :>> volume = 3.5;
		}
		part stowed : Sub[2] = (left, right);
		attribute volumes : Real[2] = stowed.volume;
	}

	part bay : Bay;
}
