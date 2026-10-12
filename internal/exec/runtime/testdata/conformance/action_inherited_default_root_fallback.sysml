package test {
	private import ScalarValues::*;

	// `default` supplies z only when no value is passed; the value does not track x.
	action def Base {
		in x : Integer = 3;
		in z : Integer default x * 2;
		out r : Integer;
	}

	action def Derived :> Base {
		first start;
		then assign x := 10;
		then assign r := z;
		then done;
	}
}
