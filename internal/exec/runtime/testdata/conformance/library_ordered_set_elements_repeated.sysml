// An OrderedSet's elements are unique (Collections::OrderedSet redefines them
// under UniqueCollection, "unique by default"), so a repeat is refused rather
// than counted or dropped: a value written twice is a uniqueness violation, and
// size answers the count of a valid OrderedSet. A List holds the same repeat.
// The repeat arrives through a feature, so the checker cannot decide it and the
// write is what refuses it.
package test {
	private import ScalarValues::*;
	private import Collections::*;
	private import CollectionFunctions::size;

	part def Holder {
		attribute one : Integer = 1;
		attribute repeated : OrderedSet { :>> elements = (one, 1, 2); }
		attribute valid : OrderedSet { :>> elements = (3, one, 2); }
		attribute count : Natural = size(valid);
		attribute list : List { :>> elements = (one, 1, 2); }
		attribute listCount : Natural = size(list);
	}

	part h : Holder;
}
