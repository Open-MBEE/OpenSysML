// `col#(i)` over an object of an OrderedCollection is CollectionFunctions::'#':
// the element at i of `col.elements`, as the qualified call answers it. An Array
// keeps its row-major `'array#'` positions, and is one member when a collection holds it.
package test {
	private import ScalarValues::*;
	private import Collections::*;
	part def Rig {
		attribute os : OrderedSet { :>> elements : Integer = (3, 5, 7); }
		attribute lst : List { :>> elements : String = ("a", "b"); }
		attribute arr : Array { :>> elements : Integer = (1, 2, 3, 4); :>> dimensions = (2, 2); }
		attribute element1 : Integer = os#(1);
		attribute element3 : Integer = os#(3);
		attribute qualified : Integer = CollectionFunctions::'#'(os, 3);
		attribute second : String = lst#(2);
		attribute cell : Integer = arr#(2, 2);
		attribute line : Array { :>> elements : Integer = (5, 6, 7); :>> dimensions = (3); }
		attribute wrapped : List { :>> elements = line; }
		attribute wrappedCell : Integer = (wrapped#(1))#(2);
		attribute nested : List { :>> elements = arr; }
		attribute nestedCell : Integer = (nested#(1))#(2, 2);
	}
}
