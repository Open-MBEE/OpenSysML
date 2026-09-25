// UniqueCollection redefines Collection::elements without `nonunique`, which
// the library notes makes it unique, and says nothing of order: its elements
// are a set, as a Set's are.
package test {
	private import Collections::*;

	attribute u : UniqueCollection { :>> elements = (3, 1, 2, 2); }
}
