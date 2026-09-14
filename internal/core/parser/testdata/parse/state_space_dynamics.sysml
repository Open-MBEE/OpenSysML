// A state-space dynamics as a user writes one against the analysis library:
// an action specializing ContinuousStateSpaceDynamics and FixedStepDynamics,
// redefining the state, input, step and stop, the derivative and output calcs,
// selecting an integrator, and declaring a zero-crossing event a state
// machine's transition accepts.
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

		perform action fall : ContinuousStateSpaceDynamics, FixedStepDynamics {
			in :>> input : FallInput = VectorOf((10.0));
			:>> stateSpace : FallState = VectorOf((19.0, 0.0));
			out :>> output : FallOutput;
			:>> timeStep = 0.5 [s];
			:>> stopTime = 10 [s];

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
			calc :>> getNextState {
				calc :>> integrate : RK4;
			}
		}

		state flight {
			entry; then falling;
			state falling;
			accept Touchdown then landed;
			state landed;
		}
	}

	action account : DiscreteStateSpaceDynamics, FixedStepDynamics {
		in :>> input : Input = VectorOf((0.05));
		:>> stateSpace : StateSpace = VectorOf((1000.0));
		:>> timeStep = 1 [s];

		calc :>> getDifference {
			in input : Input;
			in stateSpace : StateSpace;
			return : StateSpace = VectorOf((input.elements#(1) * stateSpace.elements#(1)));
		}
	}
}
