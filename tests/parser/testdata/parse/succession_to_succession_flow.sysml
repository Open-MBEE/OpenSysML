// A guarded succession may lead to a succession flow by name (the transformation's
// ObjectFlowGuard_Mapping), so the flow moves its value only when the guard holds.
package Gated {
	private import ScalarValues::*;
	action def Pass {
		attribute go : Boolean;
		action a { out y : Integer; }
		action b { in v : Integer; }
		first a if go then f;
		succession flow f of Integer from a.y to b.v;
	}
}
