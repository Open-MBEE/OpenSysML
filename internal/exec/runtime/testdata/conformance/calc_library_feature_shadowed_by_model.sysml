// A model's own feature keeps its value where it shadows a library feature of
// the same name: resolution decides, and the seam answers only for the library.
package test {
	private import ScalarValues::*;
	private import TrigFunctions::*;

	attribute pi : Real = 3.0;

	calc def approximate {
		return : Real = pi;
	}
}
