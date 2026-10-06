// A small rover the Acme modeling rules check. See README.md.
package Rover {
	port def Bus;

	part def FlightComputer {
		doc /* The computer that drives the rover. */
	}

	// No doc on SensorBus: the one element the house rules reject.
	part def SensorBus {
		out port telemetry : Bus;
	}

	part def Chassis {
		doc /* The frame the parts mount on. */
		in port control : Bus;
	}
}
