// The lunar descent of calc_rk4_lunar_descent expressed over the library's
// protocol: the state [x, h, vx, vh, m] in SI base magnitudes, the steering law
// in getDerivative, and RK4 bound to integrate. Three 1 s steps land on the
// altitude the hand-rolled RK4 computes, 15001.719185373526.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def LanderState :> StateSpace;
	attribute def LanderInput :> Input;
	attribute def LanderOutput :> Output;

	calc def Steering {
		in h : Real;
		out ax = 2000.0 * (1.0 - h / 15000.0);
		out ah = 2000.0 * (h / 15000.0);
	}

	action descent : ContinuousStateSpaceDynamics, FixedStepDynamics {
		attribute lunarGravity : Real = 1.62;
		in :>> input : LanderInput = VectorOf((0.0));
		:>> stateSpace : LanderState = VectorOf((0.0, 15000.0, 1600.0, 0.0, 1000.0));
		:>> timeStep = 1 [s];
		:>> stopTime = 3 [s];

		calc :>> getDerivative {
			in input : LanderInput;
			in stateSpace : LanderState;
			calc law : Steering { in h = stateSpace.elements#(2); }
			return : StateDerivative = VectorOf((
				stateSpace.elements#(3),
				stateSpace.elements#(4),
				law.ax / stateSpace.elements#(5),
				law.ah / stateSpace.elements#(5) - lunarGravity,
				0.0 - 1.0
			)) / 1 [s];
		}
		calc :>> getOutput {
			in input : LanderInput;
			in stateSpace : LanderState;
			return : LanderOutput = VectorOf((stateSpace.elements#(2)));
		}
		calc :>> getNextState {
			calc :>> integrate : RK4;
		}
	}
}
