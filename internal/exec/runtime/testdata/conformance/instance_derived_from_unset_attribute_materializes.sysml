// A feature derived from one nothing gives a value to holds no value, as that feature
// does: the check `-instantiate` makes over the object passes over it as it passes over the
// unset attribute, and reading either derived value is the typed no-value error naming the
// feature it is open for.
package test {
	private import ScalarValues::Real;

	part def Fitting {
		attribute offset : Real;
	}

	part def Beam {
		attribute span : Real;
		attribute halfSpan : Real = span / 2.0;
		part end : Fitting {
			attribute :>> offset = Beam::halfSpan;
		}
	}

	part beam : Beam;
}
