package StateSpaceConnections {
	private import StateSpaceRepresentation::*;

	part def Context {
		action def Apex :> ZeroCrossingEventDef;

		part pusher {
			perform action pusherBehavior : ContinuousStateSpaceDynamics {
				out :>> output;
			}
		}
		part cart {
			perform action cartBehavior : ContinuousStateSpaceDynamics {
				in :>> input;
				event occurrence apex : Apex;
			}
		}

		flow pusher.pusherBehavior.output to cart.cartBehavior.input;
	}
}
