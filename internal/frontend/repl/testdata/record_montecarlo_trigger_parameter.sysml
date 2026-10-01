package MC {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	item def s3;
	state def Cycle {
		item s32 : s3;
		entry; then idle;
		state idle;
		state busy;
		transition first idle accept s3 : s3 do action { assign s32 := s3; } then busy;
	}
	part def Probe {
		attribute t : Real;
		action settle { first start; then assign t := uniform(1.0, 5.0); then done; }
	}
	individual def probe :> Probe;
	analysis def Mc :> Simulation::MonteCarlo {
		subject analysed : Probe;
		perform action run ::> analysed.settle;
		attribute :>> observed : Real = analysed.t;
		return Mean : Real = mean;
	}
}
