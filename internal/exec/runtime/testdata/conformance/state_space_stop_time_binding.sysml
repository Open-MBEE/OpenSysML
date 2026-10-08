package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def DecayState :> StateSpace;
	attribute def DecayInput :> Input;
	attribute def DecayOutput :> Output;

	action decay : ContinuousStateSpaceDynamics, FixedStepDynamics {
		attribute rate : Real = 0.5;
		in stop : DurationValue = 2 [s];
		in :>> input : DecayInput = 0 [m] * VectorOf((0.0));
		:>> stateSpace : DecayState = 1 [m] * VectorOf((1.0));
		:>> timeStep = 0.1 [s];
		:>> stopTime = stop;

		calc :>> getDerivative {
			in input : DecayInput;
			in stateSpace : DecayState;
			return : StateDerivative = (0.0 - rate) * stateSpace / 1 [s];
		}
		calc :>> getOutput {
			in input : DecayInput;
			in stateSpace : DecayState;
			return : DecayOutput = 2.0 * stateSpace;
		}
		calc :>> getNextState {
			calc :>> integrate : Euler;
		}
	}
}
