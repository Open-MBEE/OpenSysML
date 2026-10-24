// A metadata annotation among a constraint body's steps annotates the
// constraint and is no step of it: the statements around it run in order and
// the condition is evaluated in the state they left.
package test {
	private import ScalarValues::*;

	metadata def Note { attribute tag : String; }

	constraint Bounded {
		attribute y : Real = 0.0;
		assign y := 5.0;
		@Note { tag = "bounded"; }
		y > 3.0
	}
}
