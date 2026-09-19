// Map redefines Collection::elements as unique KeyValuePairs with no order, so
// a Map's elements are a set of pairs: the same pair given twice is held once.
package test {
	private import Collections::*;

	attribute kv1 : KeyValuePair { :>> key = 1; :>> val = "a"; }
	attribute kv2 : KeyValuePair { :>> key = 2; :>> val = "b"; }
	attribute m : Map { :>> elements = (kv1, kv2, kv1); }
}
