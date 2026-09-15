// A guard that reaches zero, stays there a step, then goes negative crosses
// once: stepped by Euler, the level falls 1, 0, 0, -1, -2 over four samples
// (the rate is 0 during the second only) and Grounded is raised on the arrival
// at zero only, not while the guard rests at zero nor when it leaves. The
// machine's timeouts drive the clock: no event by 10 s would leave it in
// `missed`, a second event after the first would send it to `twice`; the one
// event takes it to `once` and the timeout from there to `settled`.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def LevelState :> StateSpace;
	attribute def LevelInput :> Input;
	attribute def LevelOutput :> Output;
	part def Gauge {
		action def Grounded :> ZeroCrossing;

		perform action level : ContinuousStateSpaceDynamics, FixedStepDynamics {
			in :>> input : LevelInput = VectorOf((1.0));
			:>> stateSpace : LevelState = VectorOf((1.0, 0.0));
			:>> timeStep = 1 [s];
			:>> stopTime = 4 [s];

			event occurrence grounded : Grounded {
				:>> guard = stateSpace.elements#(1);
			}

			calc :>> getNextState {
				calc :>> integrate : Euler;
			}
			calc :>> getDerivative {
				in input : LevelInput;
				in stateSpace : LevelState;
				return : StateDerivative = VectorOf((
					if stateSpace.elements#(2) == 1.0 ? 0.0 else 0.0 - input.elements#(1),
					1.0
				)) / 1 [s];
			}
			calc :>> getOutput {
				in input : LevelInput;
				in stateSpace : LevelState;
				return : LevelOutput = VectorOf((stateSpace.elements#(1)));
			}
		}

		state counter {
			entry; then idle;

			state idle;
			transition idle accept Grounded then once;
			transition idle accept after 10 [s] then missed;
			state missed;
			state once;
			transition once accept Grounded then twice;
			transition once accept after 10 [s] then settled;
			state twice;
			state settled;
		}
	}

	part gauge : Gauge;
}
