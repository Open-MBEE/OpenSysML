// The library declares a Set's elements "unique and unordered"
// (Collections::Set, redefining UniqueCollection::elements), so a Set holds a
// set: the elements it was given with every repeat dropped, and no order of
// its own — they enumerate in the runtime's canonical order, not as written.
package test {
	private import Collections::*;

	attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
}
