// A multi-valued feature given a default holds that default's elements when
// they conform to its multiplicity, whatever the default is written as: a
// collection literal, one value, an empty collection, or quantities.
package test {
	private import ScalarValues::Real;
	public import SI::*;

	part def Ranges {
		attribute exact : Real[3] = (1.0, 2.0, 3.0);
		attribute star : Real[0..*] = (1.0, 2.0);
		attribute empty : Real[0..*] = ();
		// One value stated for a multi-valued feature is its one element.
		attribute plus : Real[1..*] = 5.0;
		attribute masses : ISQBase::MassValue[2] = (1.0 [kg], 2.0 [kg]);
	}

	part ranges : Ranges;
}
