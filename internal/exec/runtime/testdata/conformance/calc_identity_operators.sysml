package test {
	// `===` and `!==` compare identity, which for a value is its equality.
	calc same {
		in a: Integer;
		in b: Integer;
		if a === b {
			return : Integer = 1;
		}
		if a !== b {
			return : Integer = 0;
		}
		return : Integer = -1;
	}
}
