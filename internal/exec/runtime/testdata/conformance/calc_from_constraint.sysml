package test {
	// A calc invoked from a constraint expression: the constraint's trace
	// contains the invoked calc's binding and sub-expression order.
	constraint speedWithinLimit {
		test::margin(120) > 0
	}

	calc margin {
		in speed: Integer;
		130 - speed
	}
}
