package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	part def Machine {
		attribute n : Integer = 0;
		exhibit state sm {
			entry action en assign n := n + 1;
			then s1;
			state s1 { entry assign n := n + 10; }
		}
		attribute ran : Integer = n;
		attribute smStart = sm.start;
		attribute smDone = sm.done;
		attribute smShots : Integer = size(sm.snapshots);
		attribute smSlices : Integer = size(sm.timeSlices);
		attribute smTime = Time::TimeOf(sm);
		attribute smDuration = Time::DurationOf(sm);
	}
}
