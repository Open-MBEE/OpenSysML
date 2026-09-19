// A Bag's elements are "unordered and nonunique" (Collections::Bag inherits
// the nonunique root Collection::elements), so a Bag is not a set: every
// element it was given is held, repeats included, as a sequence.
package test {
	private import Collections::*;

	attribute b : Bag { :>> elements = (3, 1, 2, 2); }
}
