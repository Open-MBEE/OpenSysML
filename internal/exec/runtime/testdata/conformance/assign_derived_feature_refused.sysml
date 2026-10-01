// A derived feature has the values the model determines from other features
// (KerML 1.0 §8.3.3.3 Feature::isDerived) — here its bound value — so an
// assignment to it is refused rather than overriding the binding.
package test {
	private import ScalarValues::*;

	action tune {
		attribute count : Integer default 3;
		derived attribute twice : Integer = count * 2;

		first start;

		action force {
			assign twice := 7;
		}

		done;

		succession first start then force;
		succession first force then done;
	}
}
