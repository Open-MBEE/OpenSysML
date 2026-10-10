// A guard written in `Base` reads `Base::x`; in a performance of `Derived` that
// feature is redefined by `Derived::x` (KerML 1.0 §7.3.4.5: the redefining
// feature takes the place of the redefined one, whose name is not inherited), so
// the inherited decision sees the redefining feature's value and takes `high`.
package test {
	private import ScalarValues::*;

	action def Base {
		attribute x : Integer default 1;
		out path : String = "";

		first start;
		decide pick;
		action low { assign path := "low"; }
		action high { assign path := "high"; }
		done;

		succession first start then pick;
		succession first pick if x < 5 then low;
		succession first pick if x >= 5 then high;
		succession first low then done;
		succession first high then done;
	}

	action def Derived :> Base {
		attribute :>> x = 10;
	}
}
