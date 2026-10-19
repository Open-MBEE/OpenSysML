package test {
	private import ScalarValues::*;
	part def Heater { attribute level : Integer = 0; }
	action def Nudge {
		in delta : Integer[1] = 1;
		in ref context : Heater[1];
		first apply;
		action apply { assign context.level := context.level + delta; }
		first apply then done;
	}
	part def Host {
		part other : Heater;
		perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
		perform action later : Nudge { in ref :>> context = other; }
		first earlier then later;
	}
	part host : Host;
}
