// `this` in a body statement of an action definition nested in a part
// definition is the occurrence the performed usage materialized: the step
// comparing it to itself runs, and the write it makes through the context
// parameter lands on the performing object.
package test {
	private import ScalarValues::*;

	part def Heater {
		attribute flag : Boolean = false;

		action def Warm {
			in ref context : Heater;
			first step;
			action step {
				assign context.flag := this == this;
			}
			first step then done;
		}

		perform action warm : Warm { in ref :>> context = this; }
	}
}
