// A nested action usage typed by state-space dynamics that binds its own state, step
// and stop on the usage performs as the same usage does standalone: the enclosing
// action's node holds the state after three 1 s steps of free fall, and its time.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def FallState :> StateSpace;
	attribute def NoInput :> Input;
	attribute def HeightOutput :> Output;

	action def FreeFall :> ContinuousStateSpaceDynamics, FixedStepDynamics {
		in :>> input : NoInput = VectorOf((0.0));
		:>> stateSpace : FallState;
		calc :>> getDerivative {
			in input : NoInput;
			in stateSpace : FallState;
			return : StateDerivative = VectorOf((stateSpace.elements#(2), -9.81)) / 1 [s];
		}
		calc :>> getOutput {
			in input : NoInput;
			in stateSpace : FallState;
			return : HeightOutput = VectorOf((stateSpace.elements#(1)));
		}
		calc :>> getNextState { calc :>> integrate : RK4; }
	}

	action def Experiment {
		action fall : FreeFall {
			:>> stateSpace = VectorOf((100.0, 0.0));
			:>> timeStep = 1 [s];
			:>> stopTime = 3 [s];
		}
	}
	action experiment : Experiment;
}
