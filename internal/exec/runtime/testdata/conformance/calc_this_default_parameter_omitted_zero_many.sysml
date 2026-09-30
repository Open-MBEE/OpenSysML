// A calc parameter's default evaluating `this` materializes the performance
// occurrence, and a later bare parameter then stores its empty binding on it:
// the occurrence holds parameters at their effective multiplicity (§7.6.3),
// [0..*] for a bare one, so the empty binding is admitted, as 
// [0..*] variant shows the same output for.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	calc def Pair {
		in self = this;
		in b : Integer[0..*];
		return : Integer = b->size();
	}
}
