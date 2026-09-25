// The startVehicle interaction of Annex A of the SysML v2 specification: parts
// performing the actions of an interaction, with anonymous messages carrying a
// payload feature between the events they perform.
package Vehicles {
	item def IgnitionCmd;
	item def EngineStatus;

	part def Driver {
		port p1;
		port p2;
	}

	part def Vehicle {
		port ignitionCmdPort;
		port statusPort;
	}

	part part0 {
		perform action startVehicle {
			action turnVehicleOn send ignitionCmd via driver.p1 {
				in ignitionCmd : IgnitionCmd;
			}
			action trigger1 accept ignitionCmd : IgnitionCmd via vehicle.ignitionCmdPort;
			flow of IgnitionCmd from trigger1.ignitionCmd to startEngine.ignitionCmd;
			action startEngine {
				in item ignitionCmd : IgnitionCmd;
				out item es : EngineStatus;
			}
			flow of EngineStatus from startEngine.es to sendStatus.es;
			action sendStatus send es via vehicle.statusPort {
				in es : EngineStatus;
			}
			action trigger2 accept es : EngineStatus via driver.p2;
		}
		part driver : Driver {
			perform startVehicle.turnVehicleOn;
			perform startVehicle.trigger2;
			event occurrence driverReady;
		}
		part vehicle : Vehicle {
			perform startVehicle.trigger1;
			perform startVehicle.sendStatus;
			event occurrence doorClosed;
		}
		first vehicle.doorClosed then driver.driverReady;
		message of ignitionCmd : IgnitionCmd from driver.turnVehicleOn to vehicle.trigger1;
		message of es : EngineStatus from vehicle.sendStatus to driver.trigger2;
	}
}

package VehicleSequenceViews {
	private import StandardViewDefinitions::*;

	// The short name of the standard sequence view definition.
	view startVehicleView : sv {
		expose Vehicles::part0;
	}
}
