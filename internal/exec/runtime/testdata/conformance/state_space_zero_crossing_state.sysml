// A lander performs the dynamics of a body dropped from 19 m under 10 m/s^2 and
// runs a state machine on the same clock. The machine's own 10 s timeout drives
// the clock, so the dynamics step beside it. The height crosses zero between the
// steps at 1.5 s and 2 s: the crossing raises Touchdown at the end of the
// detecting step, the machine's accept takes it before the timeout, and the
// terminal crossing ends the dynamics there.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def FallState :> StateSpace;
	attribute def FallInput :> Input;
	attribute def FallOutput :> Output;
	part def Lander {
		action def Touchdown :> ZeroCrossing;

		attribute gravity : Real = 10.0;

		perform action fall : ContinuousStateSpaceDynamics, FixedStepDynamics {
			in :>> input : FallInput = VectorOf((gravity));
			:>> stateSpace : FallState = VectorOf((19.0, 0.0));
			out :>> output : FallOutput;
			:>> timeStep = 0.5 [s];

			event occurrence touchdown : Touchdown {
				:>> guard = stateSpace.elements#(1);
				:>> terminal = true;
			}

			calc :>> getDerivative {
				in input : FallInput;
				in stateSpace : FallState;
				return : StateDerivative = VectorOf((stateSpace.elements#(2), 0.0 - input.elements#(1))) / 1 [s];
			}
			calc :>> getOutput {
				in input : FallInput;
				in stateSpace : FallState;
				return : FallOutput = VectorOf((stateSpace.elements#(1)));
			}
		}

		state flight {
			attribute landings : Integer = 0;
			attribute height : Real = 0.0;

			entry; then falling;

			state falling;
			accept Touchdown then landed;
			transition first falling accept after 10 [s] then lost;
			state lost;
			state landed {
				entry action {
					assign landings := landings + 1;
					assign height := fall.output.elements#(1);
				}
			}
		}
	}

	part lander : Lander;
}
