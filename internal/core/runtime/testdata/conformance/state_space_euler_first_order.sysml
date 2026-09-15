// A first-order linear system dx/dt = -k x stepped by forward Euler over the
// library's ContinuousStateSpaceDynamics: x(t) = e^(-k t), so x(2) with k = 0.5
// is e^-1 = 0.3679; Euler with h = 0.1 reaches 0.95^20 = 0.3585, a first-order error.
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
		in :>> input : DecayInput = 0 [m] * VectorOf((0.0));
		:>> stateSpace : DecayState = 1 [m] * VectorOf((1.0));
		:>> timeStep = 0.1 [s];
		:>> stopTime = 2 [s];

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
