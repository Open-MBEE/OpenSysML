// A performed action whose fork branches wait on the clock for unequal delays:
// the branch done first is held at the join while the other still waits, and
// the caller advances the clock past that wait rather than resuming the callee
// on a join it cannot pass. The join fires at t=2, the caller's own wait at t=3.
package test {
	private import SI::*;
	private import ScalarValues::*;

	action def Legs {
		out reached : Integer = 0;
		first start then f;
		fork f;
		first f then a;
		first f then b;
		action a accept after 1 [s];
		first a then a2;
		action a2 assign reached := reached + 1;
		first a2 then j;
		action b accept after 2 [s];
		first b then b2;
		action b2 assign reached := reached + 10;
		first b2 then j;
		join j;
		first j then done;
	}

	action def Caller {
		out total : Integer = 0;
		first start then legs;
		action legs : Legs;
		first legs then tail;
		action tail accept after 1 [s];
		first tail then sum;
		action sum assign total := legs.reached + 100;
		first sum then done;
	}
}
