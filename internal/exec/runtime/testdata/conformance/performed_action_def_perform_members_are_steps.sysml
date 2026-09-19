// An action def's `perform` members and nested typed actions are steps of its
// execution, not behaviors of the performance occurrence: when a part performs
// the def, each step runs in the part's frame, so `this` names the part and the
// steps write its features — as they would written directly in the part.
package test {
	private import ScalarValues::*;

	part def Station {
		attribute total : Real = 0.0;
		attribute last : Real = 0.0;
		attribute cycles : Integer = 0;

		part sensor : Sensor;

		action def Cycle {
			action def Sample {
				perform action read ::> sensor.sample;
				action sum {
					assign this.last := this.sensor.reading;
					assign this.total := this.total + this.last;
				}
				first start then read;
				first read then sum;
			}

			action sample : Sample;
			action count {
				assign this.cycles := this.cycles + 1;
			}
			first start then sample;
			first sample then count;
		}

		perform action cycle : Cycle;
	}

	part def Sensor {
		attribute reading : Real = 0.0;

		action def Read {
			action bump {
				assign this.reading := this.reading + 1.5;
			}
			first start then bump;
		}

		action sample : Read;
	}
}
