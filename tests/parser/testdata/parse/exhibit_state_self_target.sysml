// An `exhibit state` usage naming nothing — no reference subsetting, no typing
// — is its own body (SysML v2 §8.3.17, exhibitedState); its `in` members bind
// the exhibited machine's parameters, and an empty body exhibits itself too.
package test {
	item def EngineCommand;

	part def Engine {
		exhibit state modes {
			in cmd : EngineCommand = engineControl.cmd;
			entry; then a;
			state a;
			state b;
			transition first a accept EngineCommand then b;
		}
		exhibit state idle;

		port engineControl {
			in item cmd : EngineCommand;
		}
	}
}
