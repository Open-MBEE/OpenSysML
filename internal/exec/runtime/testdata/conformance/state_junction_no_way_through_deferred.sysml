package Test {
	private import ScalarValues::*;
	item def Start;

	state Machine {
		attribute stored : Boolean = false;

		entry; then wait;
		state wait {
			item deferred : Start[*] ordered;
			do action buffer {
				first start then receive;
				action receive accept kept : Start;
				then action keep { assign stored := true; }
				then receive;
			}
			transition first wait accept Start then dead;
		}
		state unused;
		junction dead;
		transition first dead if false then unused;
	}
}
