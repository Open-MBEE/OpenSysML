package test {
	private import ScalarValues::*;

	action def Statements {
		out count : Integer = 0;
		out total : Integer = 0;

		first start;
		then if count == 0 {
			assign count := 1;
		} else {
			assign count := 2;
		}
		then while count < 3 {
			assign count := count + 1;
		} until count == 3;
		then loop {
			assign count := count + 1;
		} until count == 4;
		then for i in (1, 2, 3) {
			assign total := total + i;
		}
		then done;
	}
}
