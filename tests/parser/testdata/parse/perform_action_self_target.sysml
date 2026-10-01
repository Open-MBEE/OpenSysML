// A `perform action` usage naming nothing — no reference subsetting, no typing
// — is its own body (SysML v2 §8.3.16, eventOccurrence); its `in` members bind
// the performed action's parameters, and an empty body performs itself too.
package test {
	item def EngineCommand;

	part def Engine {
		perform action rocketBoost {
			in cmd : EngineCommand = engineControl.cmd;
		}
		perform action idle;

		port engineControl {
			in item cmd : EngineCommand;
		}
	}
}
