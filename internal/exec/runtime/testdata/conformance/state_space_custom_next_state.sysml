// A getNextState the model bodies itself is the step, whatever its integrate is
// bound to: this one doubles the state each step and binds Euler, whose
// integration of the zero derivative would leave the state at 1. Three steps
// reach 8, so the body ran and the binding did not choose the scheme.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def GrowthState :> StateSpace;
	attribute def GrowthInput :> Input;
	attribute def GrowthOutput :> Output;

	action doubling : ContinuousStateSpaceDynamics, FixedStepDynamics {
		in :>> input : GrowthInput = 0 [m] * VectorOf((0.0));
		:>> stateSpace : GrowthState = 1 [m] * VectorOf((1.0));
		:>> timeStep = 1 [s];
		:>> stopTime = 3 [s];

		calc :>> getDerivative {
			in input : GrowthInput;
			in stateSpace : GrowthState;
			return : StateDerivative = 0.0 * stateSpace / 1 [s];
		}
		calc :>> getOutput {
			in input : GrowthInput;
			in stateSpace : GrowthState;
			return : GrowthOutput = 1.0 * stateSpace;
		}
		calc :>> getNextState {
			in input : GrowthInput;
			in stateSpace : GrowthState;
			in timeStep : DurationValue;
			calc :>> integrate : Euler;
			return : GrowthState = 2.0 * stateSpace;
		}
	}
}
