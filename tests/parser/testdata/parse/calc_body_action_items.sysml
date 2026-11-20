// CalculationBodyItem admits every ActionBodyItem: a calculation body states
// action usages, perform, send, accept, terminate and assignment steps.
package CalcBodyActionItems {
	private import ScalarValues::*;

	item def Ping { attribute v : Integer; }
	action def Scale { in a : Integer; out r : Integer = a * 10; }

	part def Host {
		attribute total : Integer := 0;
		part peer : Host;

		calc steps {
			in n : Integer;
			attribute x : Integer := n;
			action bump { assign x := x + 1; }
			perform action scaled : Scale { in a = n; out r; }
			send new Ping(n) to peer;
			accept Ping;
			action heard accept p : Ping;
			action wait accept q : Ping via peer;
			assign total := total + heard.p.v;
			assign peer.total := x;
			if x > 10 { terminate; }
			action stop terminate wait;
			return : Integer = x + scaled.r;
		}
	}
}
