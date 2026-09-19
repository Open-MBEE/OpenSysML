package test {
	// Two guarded successions out of one ordinary action node, written in the
	// Guarded successions select which one the token takes, without a decision
	// node.
	action route {
		attribute level : Integer = 2;
		attribute low : Integer = 0;
		attribute high : Integer = 0;

		action check assign level := level * 2;
		action alert assign high := 1;
		action idle assign low := 1;

		first check;
		succession first check if level > 10 then alert;
		succession first check if level <= 10 then idle;
	}
}
