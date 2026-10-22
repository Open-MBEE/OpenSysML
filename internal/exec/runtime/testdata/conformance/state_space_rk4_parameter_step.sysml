// The dynamics' timeStep is bound to an `in` parameter of the action rather
// than a literal: the parameter's single duration is the step the clock advances
// by and the state's rate is scaled by.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def DecayState :> StateSpace;
	attribute def DecayInput :> Input;
	attribute def DecayOutput :> Output;

	action decay : ContinuousStateSpaceDynamics, FixedStepDynamics {
		in dt : DurationValue = 0.1 [s];
		attribute rate : Real = 0.5;
		in :>> input : DecayInput = 0 [m] * VectorOf((0.0));
		:>> stateSpace : DecayState = 1 [m] * VectorOf((1.0));
		:>> timeStep = dt;
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
	}
}
