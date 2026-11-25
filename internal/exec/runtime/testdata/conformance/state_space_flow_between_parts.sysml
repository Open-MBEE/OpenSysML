// The shape of the OMG CartSample.sysml completed to run: a flow in the context carries the pusher's
// output to the cart's input, so 6 N on 2 kg for 2 s is 6 m/s; each dynamics states its own step and stop.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::DurationValue;
	private import VectorFunctions::*;
	private import StateSpaceRepresentation::*;

	attribute def PusherState :> StateSpace;
	attribute def PusherInput :> Input;
	attribute def PusherOutput :> Output;
	attribute def CartState :> StateSpace;
	attribute def CartInput :> Input;
	attribute def CartOutput :> Output;

	part def Context {
		part pusher {
			attribute pusherForce : Real = 6.0;

			perform action pusherBehavior : ContinuousStateSpaceDynamics {
				in :>> input : PusherInput = VectorOf((0.0));
				:>> stateSpace : PusherState = VectorOf((0.0));
				out :>> output : PusherOutput;
				attribute timeStep : DurationValue = 0.5 [s];
				attribute stopTime : DurationValue = 2 [s];

				calc :>> getDerivative {
					in input : PusherInput;
					in stateSpace : PusherState;
					return : StateDerivative = VectorOf((0.0)) / 1 [s];
				}
				calc :>> getOutput {
					in input : PusherInput;
					in stateSpace : PusherState;
					return : PusherOutput = VectorOf((pusherForce));
				}
			}
		}

		part cart {
			attribute mass : Real = 2.0;

			perform action cartBehavior : ContinuousStateSpaceDynamics {
				in :>> input : CartInput;
				:>> stateSpace : CartState = VectorOf((0.0));
				out :>> output : CartOutput;
				attribute timeStep : DurationValue = 0.5 [s];
				attribute stopTime : DurationValue = 2 [s];

				calc :>> getDerivative {
					in input : CartInput;
					in stateSpace : CartState;
					return : StateDerivative = input / mass / 1 [s];
				}
				calc :>> getOutput {
					in input : CartInput;
					in stateSpace : CartState;
					return : CartOutput = 1.0 * stateSpace;
				}
			}
		}

		flow pusher.pusherBehavior.output to cart.cartBehavior.input;

		state drive {
			attribute velocity : Real = 0.0;
			attribute force : Real = 0.0;

			entry; then pushing;

			state pushing;
			transition first pushing accept after 3 [s] then pushed;
			state pushed {
				entry action {
					assign velocity := cart.cartBehavior.output.elements#(1);
					assign force := cart.cartBehavior.input.elements#(1);
				}
			}
		}
	}

	part context : Context;
}
