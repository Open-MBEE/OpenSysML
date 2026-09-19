package test {
	private import ScalarValues::*;

	// `terminate c1;` written in c1's own body names the node it is in: c1 ends
	// and the parent continues along c1's succession.
	action host {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;

		first start;
		then action c1 {
			assign x := 1;
			terminate c1;
		}
		then action c2 { assign y := 2; }
		then done;
	}
}
