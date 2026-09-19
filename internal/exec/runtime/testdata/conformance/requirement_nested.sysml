package test {
	requirement ParentReq {
		require constraint { 10 < 20 }
		
		requirement NestedReq {
			require constraint { 5 < 10 }
		}
	}
}
