// The attributes a client reads off a symbol: typed, untyped, quantity-valued,
// string-valued, one with no constant default, plus one inherited and one
// redefined so masking is visible.
package demo {
	private import ScalarValues::*;

	part def Base {
		attribute mass : ISQ::MassValue default = 1500.0 [SI::kg];
		attribute label : String = "base";
		attribute inheritedOnly : Integer = 3;
	}

	part def Car :> Base {
		attribute redefines mass = 1600.0 [SI::kg];
		attribute wheels : Integer = 4;
		attribute ratio : Real = 1.5 + 0.5;
		attribute derived : Real = wheels * 2;
		part engine : Engine;
	}

	part def Engine;
}
