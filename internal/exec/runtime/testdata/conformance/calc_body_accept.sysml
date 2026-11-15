// An `accept` in a calculation body takes the message sent to its performer, and
// the steps after it read the payload (SysML v2 7.19.1: a calculation is an action).
package test {
	private import ScalarValues::*;

	item def Ping { attribute v : Integer = 0; }

	part def Host {
		calc def Heard {
			in n : Integer = 3;
			attribute x : Integer := n;
			send new Ping(7) to self;
			accept p : Ping;
			assign x := x + p.v;
			return : Integer = x;
		}
	}
}
