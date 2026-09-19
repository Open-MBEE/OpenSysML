package test {
	attribute userId : Integer = 42;
	
	requirement UserAuth {
		actor user = userId;
		require constraint { user > 0 }
	}
}
