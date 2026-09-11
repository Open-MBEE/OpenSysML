// An OrderedMap's elements are ordered and unique (Collections::OrderedMap
// redefines Map::elements, "unique by default", under OrderedCollection), so
// the same pair given twice is a uniqueness violation where a Map, holding a
// set, drops the repeat; distinct pairs keep the order written.
package test {
	private import ScalarValues::*;
	private import Collections::*;
	private import CollectionFunctions::size;

	attribute kv1 : KeyValuePair { :>> key = 1; :>> val = "a"; }
	attribute kv2 : KeyValuePair { :>> key = 2; :>> val = "b"; }

	part def Holder {
		attribute repeated : OrderedMap { :>> elements = (kv1, kv2, kv1); }
		attribute valid : OrderedMap { :>> elements = (kv2, kv1); }
		attribute count : Natural = size(valid);
		attribute asMap : Map { :>> elements = (kv1, kv2, kv1); }
		attribute mapCount : Natural = size(asMap);
	}

	part h : Holder;
}
