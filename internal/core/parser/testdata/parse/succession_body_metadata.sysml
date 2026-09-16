// A target succession's body may carry a metadata usage (SysML.xtext:1698,
// MetadataUsage as a body member), the form by which a succession out of a
// decision node states its weight.
package SuccessionBodyMetadata {
	private import Stochastic::*;
	action route {
		first start;
		then decide select;
		first select then fast { @Probability { p = 0.7; } }
		first select then slow { @Probability { p = 0.3; } }
		action fast;
		then done;
		action slow;
		then done;
	}
}
