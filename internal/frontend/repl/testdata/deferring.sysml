// A machine whose busy state keeps a signal it does not accept through the
// standard deferred-signal encoding (an ordered buffer its do action's accept
// loop fills, which its exit action sends back to self), and a later state
// accepts it; Noise is neither accepted nor kept anywhere.
package Deferring {
	item def Ping;
	attribute def Go;
	attribute def Noise;

	state def Worker {
		entry; then busy;
		state busy {
			item deferred : Ping[*] ordered;
			do action buffer {
				first start then receive;
				action receive accept kept : Ping;
				then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }
				then receive;
			}
			exit action flush {
				for kept in deferred { send kept to self; }
				then action clear { assign deferred := (); }
			}
		}
		transition busy_ready first busy accept Go then ready;
		state ready;
		transition ready_finished first ready accept Ping then finished;
		state finished;
	}

	part def Server {
		exhibit state worker : Worker;
	}

	part server : Server;
}
