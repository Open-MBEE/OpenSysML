// A shadowing declaration in the namespace that declares the import is named
// with the unit it hid too, which is where a model most often writes one.
package test {
	public import SI::*;

	attribute m : ScalarValues::Real = 4.0;

	calc def Height {
		return : ISQBase::LengthValue = 500.0 [m];
	}
}
