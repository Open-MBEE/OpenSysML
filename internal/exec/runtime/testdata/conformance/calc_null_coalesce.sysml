package test {
	// `??` evaluates its right operand only when the left one is null, which a
	// parameter admitting no value ([0..1]) may be bound to.
	calc orDefault {
		in n: Integer[0..1];
		n ?? 0
	}
}
