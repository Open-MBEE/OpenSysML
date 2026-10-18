package Lints {
	private import ScalarValues::*;
	attribute x : Real = 0.1;
	state def M { entry; then a; state a; state b; transition first a when Pnig then b; }
}
