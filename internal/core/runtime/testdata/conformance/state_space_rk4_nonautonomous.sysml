// A non-autonomous system dx/dt = t stepped by RK4 with h = 1 s: x(t) = t^2/2, so
// x(2) = 2 exactly, which RK4 reaches only when its midpoint stages read t + h/2
// and its last reads t + h; stages all reading the step's start would give 1.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def RampState :> StateSpace;
	attribute def RampInput :> Input;
	attribute def RampOutput :> Output;

	action ramp : ContinuousStateSpaceDynamics, FixedStepDynamics {
		in :>> input : RampInput = 0 [m] * VectorOf((0.0));
		:>> stateSpace : RampState = 0 [m] * VectorOf((0.0));
		:>> timeStep = 1 [s];
		:>> stopTime = 2 [s];

		calc :>> getDerivative {
			in input : RampInput;
			in stateSpace : RampState;
			return : StateDerivative = 1 [m] * VectorOf((time / 1 [s])) / 1 [s];
		}
		calc :>> getOutput {
			in input : RampInput;
			in stateSpace : RampState;
			return : RampOutput = 1.0 * stateSpace;
		}
	}
}
