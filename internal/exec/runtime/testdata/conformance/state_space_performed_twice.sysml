package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def FallState :> StateSpace;
	attribute def NoInput :> Input;
	attribute def HeightOutput :> Output;

	action fall : ContinuousStateSpaceDynamics, FixedStepDynamics {
		in :>> input : NoInput = VectorOf((0.0));
		:>> stateSpace : FallState = VectorOf((100.0, 0.0));
		:>> timeStep = 1 [s];
		:>> stopTime = 3 [s];

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
}
