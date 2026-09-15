// The first-order linear system dx/dt = -k x under the default integrator: a
// model that binds no integrate steps by classical Runge-Kutta, and x(2) with
// k = 0.5, h = 0.1 lands within 3e-8 of e^-1 = 0.36787944, where Euler misses by 9e-3.
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
	}
}
