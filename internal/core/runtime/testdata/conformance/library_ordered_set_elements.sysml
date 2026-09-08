// An OrderedSet's elements are "unique and ordered" (Collections::OrderedSet
// redefines them `ordered`), so their order is part of the value: they are
// held as a sequence in the order given, not as a set.
package test {
	private import Collections::*;

	attribute os : OrderedSet { :>> elements = (3, 1, 2); }
}
