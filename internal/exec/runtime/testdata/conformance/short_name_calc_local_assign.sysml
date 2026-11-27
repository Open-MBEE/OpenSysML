package test {
	private import ScalarValues::*;

	// A calculation's parameter, its body's local and a loop variable in it are each
	// one binding under their name and their short name.
	calc def Total {
		in <n> count : Integer;
		attribute <a> acc : Integer := 0;
		for <e> element in 1..count {
			assign a := acc + e;
			then assign e := element * 10;
			then assign acc := a + element;
		}
		then assign n := count + 1;
		return : Integer = acc + n + count;
	}
}
