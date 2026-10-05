package test {
	private import ScalarValues::*;
	item def Go;

	constraint def Ok {
		attribute y : Integer := 1;
		assign y := y * 10;
		assign y := y + 2;
		y == 12
	}

	state def TransitionGuard {
		entry; then start;
		state start;
		state accepted;
		state rejected;

		transition first start accept Go if Ok() then accepted;
	}
}
