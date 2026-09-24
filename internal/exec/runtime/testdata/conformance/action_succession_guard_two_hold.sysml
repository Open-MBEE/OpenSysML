package test {
	// Two guarded successions out of one ordinary action node whose guards both
	// hold: which one wins is not stated by the notation, so neither is chosen and
	// the ambiguity is reported. A model that means a choice writes a decision.
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
