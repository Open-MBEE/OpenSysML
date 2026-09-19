// A machine whose busy state defers a signal it does not accept, and a later
// state accepts it; Noise is neither accepted nor deferred anywhere.
package Deferring {
	attribute def Ping;
	attribute def Go;
	attribute def Noise;

	state def Worker {
		entry; then busy;
		state busy {
			defer Ping;
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
