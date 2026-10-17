// An object of a usage whose feature value names that feature by qualified name
// reads the cycle as the model-level read does: the feature's value is the cyclic
// feature value error, and a sibling read by qualified name is unaffected.
package test {
	private import ScalarValues::*;

	part self {
		attribute a : Real = test::self::a;
		attribute base : Real = 2.0;
		attribute plusOne : Real = test::self::base + 1.0;
	}
}
