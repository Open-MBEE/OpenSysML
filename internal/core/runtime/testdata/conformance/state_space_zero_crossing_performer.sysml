// Two landers perform the same falling dynamics from different heights, and each
// exhibits a machine accepting its own performance's touchdown event. The low
// lander touches down at 2 s; the high one, dropped from 1000 m, has not by the
// time its 10 s timeout fires. The high machine runs first and drives the clock
// past the low lander's crossing: the crossing it sees is not its own, so it is
// lost, and the low machine lands on the touchdown of its own fall.
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
		attribute height : Real default = 19.0;
		attribute landings : Integer = 0;

		perform action fall : ContinuousStateSpaceDynamics, FixedStepDynamics {
			in :>> input : FallInput = VectorOf((gravity));
			:>> stateSpace : FallState = VectorOf((height, 0.0));
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

		exhibit state flight {
			entry; then falling;

			state falling;
			transition first falling accept :> fall.touchdown then landed;
			transition first falling accept after 10 [s] then lost;
			state lost;
			state landed {
				entry action {
					assign landings := landings + 1;
				}
			}
		}
	}

	part fleet {
		part low : Lander;
		part high : Lander {
			:>> height = 1000.0;
		}
	}
}
