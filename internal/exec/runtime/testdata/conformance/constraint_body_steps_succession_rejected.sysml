// A succession is a stated flow: a verdict orders its steps by declaration
// rather than by the successions stated, so the flow is refused.
package test {
	private import ScalarValues::*;

	part def Rig {
		attribute z : Real = 1;
		action a; action b;
		constraint flowed { first a then b; z > 0 }
	}
}
