package test {
	// A `then` attached to a member declares a succession, so execution follows
	// the edges rather than declaration order: the members below are declared
	// third, second, fourth but run second -> fourth -> third, so `t` counts up to 243.
	// The member-attached `then` is the only thing sequencing `fourth` after
	// `second`; ignoring it leaves `second` with no successor.
	action ordered {
		attribute t : Integer = 0;

		first start;

		action third assign t := t * 10 + 3;

		action second assign t := t * 10 + 2;
		then action fourth assign t := t * 10 + 4;

		done;

		succession first start then second;
		succession first fourth then third;
		succession first third then done;
	}
}
