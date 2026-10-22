// An action nested in an analysis body, typed by state-space dynamics and binding its
// own state, step and stop, is performed as dynamics: a free fall from 100 m ends at
// 55.855 m after three 1 s steps, and the result reads the state the run left, not
// the bound initial state.
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

	analysis def Drop {
		action fall : FreeFall {
			:>> stateSpace = VectorOf((100.0, 0.0));
			:>> timeStep = 1 [s];
			:>> stopTime = 3 [s];
		}
		return finalHeight : Real = fall.stateSpace.elements#(1);
	}
	analysis drop : Drop;
}
