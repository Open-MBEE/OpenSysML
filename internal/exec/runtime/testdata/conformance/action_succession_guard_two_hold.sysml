package test {
	// Two guarded successions out of one ordinary action node whose guards both
	// hold. Each guarded succession is its own transition performance whose guard
	// constrains only its own HappensBefore link (TransitionPerformances.kerml,
	// NonStateTransitionPerformance, transitionLink : HappensBefore[0..1]), and
	// only a decision makes its successions exclusive (DecisionPerformance). So
	// neither link excludes the other: both alert and idle are performed after check.
	action route {
		attribute level : Integer = 12;
		attribute low : Integer = 0;
		attribute high : Integer = 0;

		action check assign level := level;
		action alert assign high := 1;
		action idle assign low := 1;

		first check;
		succession first check if level > 10 then alert;
		succession first check if level > 5 then idle;
	}
}
