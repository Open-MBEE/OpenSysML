// An `exhibit state` usage naming nothing — no reference subsetting, no typing
// — exhibits itself (SysML v2 §8.3.17, exhibitedState), and its `in` member
// binds the machine's parameter from the object's port.
package test {
	item def EngineCommand;

	port def CmdPort {
		in item cmd : EngineCommand;
	}

	part def Engine {
		port engineControl : CmdPort;

		exhibit state modes {
			in cmd : EngineCommand = engineControl.cmd;

			entry; then start;
			state start;
			state waiting;
			state done;

			succession first start then waiting;

			transition first waiting
				accept EngineCommand
				then done;
		}
	}

	part engine : Engine;
}
