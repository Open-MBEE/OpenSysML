// What a set answers: two Sets are equal whatever order their elements were
// given in and however often, and so are their elements; size counts distinct
// elements; contains and containsAll are membership; the empty set is empty.
// A set flowing into any other multi-valued feature — ordered, nonunique or a
// plain `Integer[*]`, all of which the runtime holds as sequences — becomes the
// sequence of its elements in canonical order; a Bag or OrderedSet is never a set.
package test {
	private import ScalarValues::*;
	private import Collections::*;
	private import CollectionFunctions::*;

	part def Holder {
		attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
		attribute t : Set { :>> elements = (2, 3, 1); }
		attribute e : Set { :>> elements = (); }
		attribute b : Bag { :>> elements = (3, 1, 2, 2); }
		attribute os : OrderedSet { :>> elements = (3, 1, 2); }

		attribute count : Natural = size(s);
		attribute emptyCount : Natural = size(e);
		attribute bagCount : Natural = size(b);
		attribute ordered : Integer[*] ordered = s.elements;
		attribute repeatable : Integer[*] nonunique = s.elements;
		attribute plain : Integer[*] = t.elements;
		attribute inOrderedSet : Integer[*] ordered = os.elements;

		assert constraint equalRegardlessOfOrder { s == t }
		assert constraint elementsEqualRegardlessOfOrder { s.elements == t.elements }
		assert constraint distinctFromTheEmptySet { s != e }
		assert constraint emptySetsAreEqual { e == e }
		assert constraint membership { contains(s, 2) and not contains(s, 5) and not contains(e, 2) }
		assert constraint allMembers { containsAll(s, t) and containsAll(t, s) and containsAll(s, e) }
		assert constraint emptiness { isEmpty(e) and notEmpty(s) and not isEmpty(s) }
		assert constraint notASequence { s != os and s.elements != b.elements }
	}

	part h : Holder;
}
