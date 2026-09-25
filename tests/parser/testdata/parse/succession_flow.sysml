// A `succession flow` is a flow usage that keeps the `succession` prefix
// (SysML.xtext SuccessionFlowUsage), in each spelling a flow admits.
package Flows {
	item def Fuel;
	action def Supply {
		action tank { out item x : Fuel; }
		action engine { in item x : Fuel; }
		succession flow from tank.x to engine.x;
		succession flow tank.x to engine.x;
		succession flow named from tank.x to engine.x;
		succession flow of fuel : Fuel from tank.x to engine.x {
			doc /* a succession flow declaring its payload */
		}
		flow from tank.x to engine.x;
	}
}
