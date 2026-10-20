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
	}
}
