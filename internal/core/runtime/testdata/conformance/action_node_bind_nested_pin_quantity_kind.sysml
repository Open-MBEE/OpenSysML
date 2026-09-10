// A quantity bound through a node into a pin of its nested flow is spelt by
// the quantity kind that pin declares even though the nested action is yet to
// start when the value arrives: a work value written in joules waits for a
// torque pin as newton metres, and for an energy pin in joules as written.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;

	action outer {
		attribute work : EnergyValue = 12 [J];
		out attribute same : Boolean;

		bind leg.inner.moment = work;
		bind leg.inner.heat = work;
		bind leg.inner.equal = same;

		first start;
		then action leg {
			first start;
			then action inner {
				in moment : TorqueValue;
				in heat : EnergyValue;
				out equal : Boolean;
				assign equal := moment == heat;
			}
			then done;
		}
		then done;
	}
}
