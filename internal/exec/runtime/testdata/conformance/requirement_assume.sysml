package test {
	requirement SafeOperation {
		assume constraint { 1 > 100 }  // false assumption: `false implies …` holds
		require constraint { 50 < 100 }  // true requirement
	}
}
