// Discrete dynamics over the library's protocol: each sample adds getDifference
// to the state. A balance compounding 5% a step reaches 1000 * 1.05^10 after ten
// samples, and a second component counts the samples taken.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def AccountState :> StateSpace;
	attribute def AccountInput :> Input;
	attribute def AccountOutput :> Output;

	action account : DiscreteStateSpaceDynamics, FixedStepDynamics {
		in :>> input : AccountInput = VectorOf((0.05));
		:>> stateSpace : AccountState = VectorOf((1000.0, 0.0));
		:>> timeStep = 1 [s];
		:>> stopTime = 10 [s];

		calc :>> getDifference {
			in input : AccountInput;
			in stateSpace : AccountState;
			return : StateSpace = VectorOf((input.elements#(1) * stateSpace.elements#(1), 1.0));
		}
		calc :>> getOutput {
			in input : AccountInput;
			in stateSpace : AccountState;
			return : AccountOutput = VectorOf((stateSpace.elements#(1)));
		}
	}
}
