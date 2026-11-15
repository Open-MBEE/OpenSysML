// An `accept` in a calculation body takes the message sent to its performer, and
// a step after it reads the payload (SysML v2 7.19.1: a calculation is an action).
package test {
	private import ScalarValues::*;

	item def Ping { attribute v : Integer; }

	part def Host {
		calc def Heard {
			in n : Integer = 3;
			attribute x : Integer := n;
			action tx send new Ping(7) to self;
			then action rx accept p : Ping;
			then assign x := x + rx.p.v;
			return : Integer = x;
		}
	}
}
