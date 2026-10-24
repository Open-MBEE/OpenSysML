package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	part def Runner {
		attribute n : Integer = 0;
		perform action act {
			first start;
			then action inc assign n := n + 1;
			then done;
		}
		attribute ran : Integer = n;
		attribute actStart = act.start;
		attribute actDone = act.done;
		attribute actShots : Integer = size(act.snapshots);
		attribute actSlices : Integer = size(act.timeSlices);
		attribute actPortions : Integer = size(act.portions);
		attribute ownShots : Integer = size(self.snapshots);
		attribute ownStart = self.start;
		attribute ownDone = self.done;
		attribute actDuration = Time::DurationOf(act);
		attribute actTime = Time::TimeOf(act);
		attribute actClockTime = Clocks::TimeOf(act);
		attribute startShotOfStart : Boolean = act.start.start == act.start;
		attribute endShotOfStart : Boolean = act.start.done == act.start;
		attribute startShots : Integer = size(act.start.snapshots);
		attribute startSnapshotOf : Boolean = includes(act.start.snapshotOf, act);
		attribute startTimeSliceOf : Integer = size(act.start.timeSliceOf);
		part p1 : P1;
		attribute shotIsP1 : Boolean = p1.start istype P1;
		attribute shotParts : Integer = p1.start.v;
	}
	part def P1 {
		part pa;
		part pb;
		attribute v : Integer = size(self.suboccurrences);
	}
}
