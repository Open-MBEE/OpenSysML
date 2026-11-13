// A performance subsetting a valued step starts from the step's value when its holder binds nothing.
package test {
	private import ScalarValues::Integer;
	action def Step { in x : Integer; out y : Integer; first start; then assign y := x + 1; }
	action t : Step { in x = 9; }
	part def P { perform action pt : Step :> t; }
	part p : P;
}
