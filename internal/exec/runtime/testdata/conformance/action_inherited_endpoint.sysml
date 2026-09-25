package test {
	private import ScalarValues::*;

	action def Base {
		out result : Integer;

		action setResult {
			assign result := 7;
		}
	}

	action def Derived :> Base {
		first start then setResult;
	}
}
