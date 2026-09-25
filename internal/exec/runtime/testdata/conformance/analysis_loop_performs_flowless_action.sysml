// An analysis body's `for` performs an action definition that states no flow,
// only computed outputs: each performance takes its inputs, binds its outputs and
// ends, and the loop accumulates them into the result's initial (`:=`) sequence.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	part def Ship {
		attribute drag : Real = 2.0;
	}
	part ship : Ship;

	action def Step {
		in force : Real;
		in drag : Real;
		in v_in : Real;
		out a_out : Real = (force - drag * v_in) / 10.0;
		out v_out : Real = v_in + a_out;
	}

	analysis def Profile {
		subject s : Ship;
		in forces : Real[*];
		in v0 : Real;
		return accelerations : Real[*] := ();
		private attribute speed := v0;
		for i in 1..forces->size() {
			perform action step : Step {
				in force = forces#(i);
				in drag = s.drag;
				in v_in = speed;
			}
			then assign speed := step.v_out;
			then assign accelerations := accelerations->including(step.a_out);
		}
	}

	analysis profile : Profile {
		subject s = ship;
		in forces = (30.0, 40.0);
		in v0 = 5.0;
	}
}
