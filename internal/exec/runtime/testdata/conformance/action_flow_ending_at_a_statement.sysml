package test {
	// The flow ends at a statement written directly in the body, which has no
	// succession out of it, so the token retires there and the action completes.
	action ends {
		attribute t : Integer = 0;

		first start;
		then action a assign t := 1;
		then assign t := t + 7;
	}
}
