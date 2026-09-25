// A Set given no elements is the empty set, not an empty sequence: elements is
// declared [0..*], so a Set that values it with nothing holds a set of nothing.
package test {
	private import Collections::*;

	attribute e : Set { :>> elements = (); }
}
