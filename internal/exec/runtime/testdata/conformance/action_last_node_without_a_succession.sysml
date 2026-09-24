package test {
	// No `done` node is declared: every action inherits one, so the flow ends at
	// `b`, which has no succession out of it, and the action completes.
	action ends {
		attribute t : Integer = 0;

		first start;
		then action a assign t := t + 4;
		then action b assign t := t * 10;
	}
}
