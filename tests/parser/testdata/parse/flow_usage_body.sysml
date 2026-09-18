// A flow usage carries a body after its ends (SysML.xtext FlowUsage), and its
// `to` may stand on a continuation line.
package Flows {
	item def Fuel;
	action def Supply {
		action tank {
			out item x : Fuel;
		}
		action engine {
			in item x : Fuel;
		}

		flow tank.x to engine.x {
			doc /* the ends the flow joins */
		}
		flow tank.x
			to engine.x;
		flow of fuel : Fuel from tank.x to engine.x {
			doc /* a flow declaring its payload */
		}
		flow named from tank.x to engine.x {
			doc /* a named flow states its ends after `from` */
		}
	}
}
