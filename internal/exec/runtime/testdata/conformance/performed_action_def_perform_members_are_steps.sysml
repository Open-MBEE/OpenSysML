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
			in ref context : Station;
			action def Sample {
				in ref context : Station;
				perform action read ::> context.sensor.sample;
				action sum {
					assign context.last := context.sensor.reading;
					assign context.total := context.total + context.last;
				}
				first start then read;
				first read then sum;
			}

			action sample : Sample { in ref :>> context = Cycle::context; }
			action count {
				assign context.cycles := context.cycles + 1;
			}
			first start then sample;
			first sample then count;
		}

		perform action cycle : Cycle { in ref :>> context = this; }
	}

	part def Sensor {
		attribute reading : Real = 0.0;

		action def Read {
			in ref context : Sensor;
			action bump {
				assign context.reading := context.reading + 1.5;
			}
			first start then bump;
		}

		action sample : Read { in ref :>> context = this; }
	}
}
