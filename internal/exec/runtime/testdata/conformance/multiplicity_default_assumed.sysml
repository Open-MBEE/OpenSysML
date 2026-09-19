// A feature that declares no multiplicity holds exactly one value, so a default
// of one value is held and one of several is reported — as it is for a feature
// that writes that bound out as `[1]`.
package test {
	private import ScalarValues::Real;

	part def Sub {
		attribute volume : Real = 2.0;
	}

	part def Craft {
		part left : Sub;
		part right : Sub {
			attribute :>> volume = 3.5;
		}
		part both : Sub[0..*] = (left, right);

		// One value, so the assumed bound holds it.
		attribute one : Real = left.volume;
		// Two values against the assumed upper bound of 1.
		attribute two : Real = both.volume;
	}

	part craft : Craft;
}
