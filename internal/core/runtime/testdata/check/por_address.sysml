// A send addressed through a reference feature meets the write that redirects
// it: which node receives the message depends on the order of the two moves.
// Each branch takes a further step before the join so the two are not already
// dependent through it.
package test {
	private import ScalarValues::*;

	item def Ping;
	port def PingPort { in item ping : Ping; }
	part def Node { port inPort : PingPort; }
	part alpha : Node;
	part beta : Node;
	part hub { ref part dest : Node = alpha; }

	action addressed {
		attribute mark : Integer = 0;
		first start;
		fork split;
		action aim { assign hub.dest := beta; }
		action aimed;
		action post { send new Ping() to hub.dest.inPort; }
		action posted { assign mark := 1; }
		join sync;
		done;
		succession first start then split;
		succession first split then aim;
		succession first split then post;
		succession first aim then aimed;
		succession first post then posted;
		succession first aimed then sync;
		succession first posted then sync;
		succession first sync then done;
	}
}
