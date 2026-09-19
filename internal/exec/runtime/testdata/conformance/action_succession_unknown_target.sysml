package test {
	// A succession naming a member the body does not declare cannot be lowered
	// into an edge, so lowering says which name it could not find rather than
	// running an action whose order is not the one written.
	action broken {
		attribute t : Integer = 0;

		first start;

		action bump assign t := t + 1;

		succession first bump then nowhere;
	}
}
