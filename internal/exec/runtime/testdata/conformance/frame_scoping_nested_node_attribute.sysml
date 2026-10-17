// A nested action node's own `attribute x` is a feature of that node, not of the
// enclosing action (KerML 1.0 §8.2.3.5.3: an owned member resolves before an
// inherited or outer one). The node's body reads and writes its own `x`; a later
// node of the enclosing body, which declares no `x`, resolves outward to the
// enclosing action's `x`, which the nested assignment never touched.
package test {
	private import ScalarValues::*;

	action def Outer {
		attribute x : Integer = 1;
		out seen : Integer = 0;
		out innerSeen : Integer = 0;

		first start;
		action inner {
			attribute x : Integer = 10;
			assign x := x + 5;
			then assign innerSeen := x;
		}
		action later {
			assign seen := x;
		}
		done;

		succession first start then inner;
		succession first inner then later;
		succession first later then done;
	}
}
