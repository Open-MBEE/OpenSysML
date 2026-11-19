// A probe advancing 1 m/s passes through a band between 0.3 m and 0.4 m inside
// one step of 1 s. The guard, the product of the distances to the band's two
// edges, is positive at both ends of the step and at every quarter of it, and
// negative only inside the band: the crossing into the band and the one out of
// it are each located inside the step, so InBand is raised at 0.3 s and again
// at 0.4 s, and the machine reads the position of each. Sampled at the steps'
// ends alone the guard never changes sign, and the machine would time out in
// `missed`.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;
	private import StateSpaceIntegration::*;

	attribute def ProbeState :> StateSpace;
	attribute def ProbeInput :> Input;
	attribute def ProbeOutput :> Output;
	part def Probe {
		action def InBand :> ZeroCrossing;

		perform action advance : ContinuousStateSpaceDynamics, FixedStepDynamics {
			in :>> input : ProbeInput = VectorOf((1.0));
			:>> stateSpace : ProbeState = VectorOf((0.0));
			out :>> output : ProbeOutput;
			:>> timeStep = 1 [s];
			:>> stopTime = 2 [s];

			event occurrence band : InBand {
				:>> guard = (stateSpace.elements#(1) - 0.3) * (stateSpace.elements#(1) - 0.4);
			}

			calc :>> getDerivative {
				in input : ProbeInput;
				in stateSpace : ProbeState;
				return : StateDerivative = VectorOf((input.elements#(1))) / 1 [s];
			}
			calc :>> getOutput {
				in input : ProbeInput;
				in stateSpace : ProbeState;
				return : ProbeOutput = VectorOf((stateSpace.elements#(1)));
			}
		}

		state passage {
			attribute entered : Real = 0.0;
			attribute left : Real = 0.0;

			entry; then outside;

			state outside;
			transition outside accept InBand then inside;
			transition outside accept after 10 [s] then missed;
			state missed;
			state inside {
				entry action {
					assign entered := advance.output.elements#(1);
				}
			}
			transition inside accept InBand then beyond;
			transition inside accept after 10 [s] then stuck;
			state stuck;
			state beyond {
				entry action {
					assign left := advance.output.elements#(1);
				}
			}
		}
	}

	part probe : Probe;
}
