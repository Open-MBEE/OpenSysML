// The parser must tell the forms `assert` admits apart, so it attempts the
// nested `constraint` declaration and rewinds to the reference form when the
// word after the keyword is not `constraint`.
constraint def Positive {
	in attribute x : Real;
	x > 0
}

constraint def BacktrackedConditions {
	in attribute x : Real;

	// Reference forms: the attempt at a nested declaration rewinds.
	assert Positive;
	assert not Positive;

	// Declaration forms, with and without a name.
	assert constraint positive {
		x > 0
	}
	assert constraint {
		x < 10
	}

	// Semicolons are optional between conditions, so two conditions read as two
	// members rather than one.
	x > 0
	x < 100
}
