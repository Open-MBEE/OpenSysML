// An invoked node's output returns to the same-named feature of the performance
// enclosing it once the node completes. That feature's occurrence is under way,
// so the return is a write, and a constant feature keeps one value over the
// lifetime of its featuring occurrence (KerML 1.0 §8.3.3.3 Feature::isConstant):
// returning Raise's limit into Tune's constant limit is refused.
package test {
	private import ScalarValues::*;

	action def Raise {
		out limit : Integer;

		first start;
		then action raising { assign limit := 7; }
		then done;
	}

	action def Tune {
		constant attribute limit : Integer = 5;

		first start;
		then action raised : Raise;
		then done;
	}
}
