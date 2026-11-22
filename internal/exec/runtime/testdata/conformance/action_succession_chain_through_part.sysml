// A succession end may be a feature chain (§7.17.4): `vehicle.doorClosed` is performed on the
// part `vehicle` where the successions put it, once, after `start` and before `driverReady`.
package P {
	private import ScalarValues::*;

	part def Vehicle {
		attribute n : Integer = 0;
		action doorClosed { assign n := n + 1; }
	}
	part def Driver {
		attribute m : Integer = 0;
		action driverReady { assign m := m + 10; }
	}

	action def Startup {
		part vehicle : Vehicle;
		part driver : Driver;
		out result : Integer;
		action read { assign result := vehicle.n + driver.m; }

		first start then vehicle.doorClosed;
		first vehicle.doorClosed then driver.driverReady;
		succession ready first driver.driverReady then read;
		first read then done;
	}
}
