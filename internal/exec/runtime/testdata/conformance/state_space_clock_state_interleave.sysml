// A tank performs first-order cooling dx/dt = -2 x by forward Euler in 0.25 s
// steps and exhibits a machine sampling the temperature on timed accepts, all on
// one clock. The sample at 0.375 s reads the state after the step at 0.25 s. At
// 0.75 s the machine's accept and a step are due together: a due-order choice
// decides which runs first, so the late sample is x(0.5) = 0.25 or x(0.75) =
// 0.125 — under the default policy the machine, created after the probe, runs
// first. The probe reads the state after the dynamics stop at 1 s.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def Temperature :> StateSpace;
	attribute def Ambient :> Input;
	attribute def Reading :> Output;

	part def Tank {
		attribute early : Real = 0.0;
		attribute late : Real = 0.0;

		perform action cool : ContinuousStateSpaceDynamics, FixedStepDynamics {
			in :>> input : Ambient = VectorOf((0.0));
			:>> stateSpace : Temperature = VectorOf((1.0));
			out :>> output : Reading;
			:>> timeStep = 0.25 [s];
			:>> stopTime = 1 [s];

			calc :>> getDerivative {
				in input : Ambient;
				in stateSpace : Temperature;
				return : StateDerivative = (0.0 - 2.0) * stateSpace / 1 [s];
			}
			calc :>> getOutput {
				in input : Ambient;
				in stateSpace : Temperature;
				return : Reading = VectorOf((stateSpace.elements#(1)));
			}
			calc :>> getNextState {
				calc :>> integrate : Euler;
			}
		}

		exhibit state monitor {
			entry; then waiting;

			state waiting;
			accept after 0.375 [s] then sampledEarly;
			state sampledEarly {
				entry assign early := cool.output.elements#(1);
			}
			transition first sampledEarly accept after 0.375 [s] then sampledLate;
			state sampledLate {
				entry assign late := cool.output.elements#(1);
			}
		}
	}

	part tank : Tank;

	action probe {
		attribute early : Real = 0.0;
		attribute late : Real = 0.0;
		attribute final : Real = 0.0;

		first start;
		then action arm assign early := tank.early;
		then action wait accept after 1.1 [s];
		then action read {
			assign early := tank.early;
			assign late := tank.late;
			assign final := tank.cool.output.elements#(1);
		}
		then done;
	}
}
