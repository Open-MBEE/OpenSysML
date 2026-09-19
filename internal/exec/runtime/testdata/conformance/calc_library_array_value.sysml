// An Array is one value: a calc returns it whole, dimensions and elements, into
// a result typed by Array.
package test {
	private import ScalarValues::*;
	private import Collections::*;

	attribute a : Array { :>> dimensions = (2, 2); :>> elements = (1, 2, 3, 4); }

	calc def whole {
		return : Array = a;
	}
}
