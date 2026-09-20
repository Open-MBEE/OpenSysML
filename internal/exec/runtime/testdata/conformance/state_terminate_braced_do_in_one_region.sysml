// A `terminate` in one region's braced `do { … }` ends that block only: the
// sibling region's do block runs every statement of its own, each region's
// state completes when its do behavior is over, and both reach their final states.
package Test {
	private import ScalarValues::*;

	state def Machine parallel {
		attribute a : Integer = 0;
		attribute b : Integer = 0;

		state left {
			entry; then lwork;
			state lwork {
				do {
					assign a := 1;
					terminate;
					assign a := 9;
				}
			}
			state ldone;
			succession first lwork then ldone;
		}

		state right {
			entry; then rwork;
			state rwork {
				do {
					assign b := 1;
					assign b := b + 1;
					assign b := b + 1;
				}
			}
			state rdone;
			succession first rwork then rdone;
		}
	}
}
