// A Set given elements that are already distinct holds exactly those: nothing
// is dropped, and the set is the same set whatever order they were written in.
package test {
	private import Collections::*;

	attribute s : Set { :>> elements = (2, 3, 1); }
}
