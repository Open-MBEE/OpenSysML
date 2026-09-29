// A bare `in` parameter of a calc is optional the same way an explicit
// [0..*] is (§7.6.3): Count() binds nothing to `x`, which the body reads as
// the empty sequence, and answers 0 as the [0..*] variant does.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc count { in x : Integer; x->size() }
}
