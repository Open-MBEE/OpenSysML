package T {
	part def PQ;
	action def At { in t : Time::TimeInstantValue; action aat accept at t; }
	action def After { in d : ISQ::DurationValue; part q : PQ; action aaf accept after d via q; }
}
