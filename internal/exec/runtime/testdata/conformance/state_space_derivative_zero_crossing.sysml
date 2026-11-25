// A thrown ball whose apex is a guardless ZeroCrossingEventDef event: it occurs where the derivative
// of the height (7.5, 2.5, -2.5) crosses zero (SysML v2 9.4.4.2), inside that step, at t = 0.75 s.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::DurationValue;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;

	attribute def BallState :> StateSpace;
	attribute def BallInput :> Input;
	attribute def BallOutput :> Output;

	part def Ball {
		action def Apex :> ZeroCrossingEventDef;

		perform action flight : ContinuousStateSpaceDynamics {
			in :>> input : BallInput = VectorOf((10.0));
			:>> stateSpace : BallState = VectorOf((0.0, 7.5));
			out :>> output : BallOutput;
			attribute timeStep : DurationValue = 0.5 [s];
			attribute stopTime : DurationValue = 2 [s];

			event occurrence apex : Apex;

			calc :>> getDerivative {
				in input : BallInput;
				in stateSpace : BallState;
				return : StateDerivative = VectorOf((stateSpace.elements#(2), 0.0 - input.elements#(1))) / 1 [s];
			}
			calc :>> getOutput {
				in input : BallInput;
				in stateSpace : BallState;
				return : BallOutput = VectorOf((stateSpace.elements#(1)));
			}
		}

		state watch {
			attribute apexes : Integer = 0;
			attribute height : Real = 0.0;

			entry; then rising;

			state rising;
			accept Apex then falling;
			transition first rising accept after 10 [s] then lost;
			state lost;
			state falling {
				entry action {
					assign apexes := apexes + 1;
					assign height := flight.output.elements#(1);
				}
			}
		}
	}

	part ball : Ball;
}
