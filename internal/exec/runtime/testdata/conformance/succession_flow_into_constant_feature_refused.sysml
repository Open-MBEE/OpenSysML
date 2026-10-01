// A succession flow drops its payload off at an input feature of its target
// (Transfers.kerml, FlowTransfer), so a flow into a constant input
// (KerML 1.0 §8.3.3.3 Feature::isConstant) is refused like an assignment.
package test {
	private import ScalarValues::*;

	action def Source { out attribute v : Integer; }
	action def Sink { in constant attribute level : Integer; }

	action pipe {
		first start;

		action src : Source { out :>> v = 3; }
		action snk : Sink;

		succession flow src.v to snk.level;

		succession first start then src;
	}
}
