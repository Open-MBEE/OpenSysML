package test {
	requirement SafeOperation {
		assume constraint { 1 > 100 }  // false assumption (trusted, doesn't fail)
		require constraint { 50 < 100 }  // true requirement
	}
}
