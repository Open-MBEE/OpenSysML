// A flow drops its payload off at an input feature of its target
// (Transfers.kerml, FlowTransfer), so a flow into an end feature — constant as
// one that may vary in time (SysML 2.0 §8.4.2.2) — is refused like an
// assignment.
package test {
	private import ScalarValues::*;

	action def Source { out attribute v : Integer; }
	action def Sink { end attribute level : Integer; }

	action pipe {
		first start;

		action src : Source { out :>> v = 3; }
		action snk : Sink;

		flow src.v to snk.level;

		succession first start then src;
		succession first src then snk;
	}
}
