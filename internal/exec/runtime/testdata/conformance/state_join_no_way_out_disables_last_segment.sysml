package Test {
	private import ScalarValues::*;
	attribute def Another;

	state Machine {
		attribute value : Integer = 15;
		attribute log : String = "";

		entry; then S1;
		state S1 parallel {
			state left {
				entry; then a;
				state a;
				transition first a do assign log := log + "T1.2 " then j;
			}
			state right {
				entry; then b;
				state b;
				transition first b do assign log := log + "T1.4 " then j;
			}
			join j;
			transition first j if value < 10 then S2;
		}
		state S2;
		state S3;
		transition first S1 accept Another do assign log := log + "T5" then S3;
	}
}
