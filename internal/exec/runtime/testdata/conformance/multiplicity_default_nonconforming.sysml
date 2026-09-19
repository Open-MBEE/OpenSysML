// A default whose element count does not conform to the feature's multiplicity
// is reported, rather than broadcast to fill the lower bound, truncated to the
// upper one, or dropped. Every default here is an expression, whose count only
// materializing it knows; a count the type tier sees statically is reported
// there instead (passes.TestValueTooManyValuesForUpperBound).
package test {
	private import ScalarValues::Real;

	part def Sub {
		attribute volume : Real;
	}

	part def Bad {
		part left : Sub {
			attribute :>> volume = 2.0;
		}
		part right : Sub {
			attribute :>> volume = 3.5;
		}
		part both : Sub[0..*] = (left, right);
		part empty : Sub[0..*] = ();

		attribute few : Real[3] = both.volume;
		attribute many : Real[0..1] = both.volume;
		attribute none : Real[1..3] = empty.volume;
		part stowed : Sub[3] = both;
	}

	part bad : Bad;
}
