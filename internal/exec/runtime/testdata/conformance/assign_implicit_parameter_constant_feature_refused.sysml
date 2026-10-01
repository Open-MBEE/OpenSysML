// A parameter of a specialized behavior redefines the parameter at its position
// in the general one without a written `:>>` (KerML 1.0 §7.4.7.2), and a feature
// redefining a constant feature is constant too (KerML 1.0 §8.3.3.3,
// validateSubsettingConstantConformance). Writing Spec's cap, which implicitly
// redefines Gen's constant limit, is refused naming the feature it is constant by.
package test {
	private import ScalarValues::*;

	action def Gen {
		in constant attribute limit : Integer;
	}

	action def Spec :> Gen {
		in attribute cap : Integer = 5;
	}

	action tune : Spec {
		first start;
		then action raise {
			assign cap := 7;
		}
		then done;
	}
}
