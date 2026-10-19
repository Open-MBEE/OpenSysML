package test {
	private import ScalarValues::*;
	part def Heater { attribute level : Integer = 0; }
	action def Nudge {
		in delta : Integer[1] = 1;
		in ref context : Heater[1];
		out level : Integer;
		first apply;
		action apply { assign context.level := context.level + delta; assign level := context.level; }
		first apply then done;
	}
	part def Host {
		part other : Heater;
		perform action later : Nudge { in ref :>> context = other; }
	}
	part host : Host;
}
