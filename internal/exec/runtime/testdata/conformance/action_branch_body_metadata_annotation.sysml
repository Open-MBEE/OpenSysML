// A metadata annotation inside a branch body annotates the branch and is no
// step of it: the assignment beside it runs and the output holds its value.
package test {
	private import ScalarValues::*;

	metadata def Note { attribute tag : String; }

	action def Count {
		out n : Integer := 0;
		if true {
			@Note { tag = "taken"; }
			assign n := 1;
		}
	}
}
