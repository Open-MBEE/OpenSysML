package test {
	// Oracle (docs/project/behavior-semantic-oracle.md): `bump` is reached from `start`
	// and from the decision behind it. Each arrival is a pass of the loop and the other
	// succession's source cannot perform before this one, so `bump` runs once per pass: n = 3.
	action count {
		attribute n : Integer = 0;

		action bump { assign n := n + 1; }
		decide again;

		succession first start then bump;
		succession first bump then again;
		succession first again if n < 3 then bump;
		succession first again if n >= 3 then done;
	}
}
