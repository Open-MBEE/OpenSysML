// A write to a feature a parked trigger's condition reads: the waiting branch
// resumes only once the writer has run, and the counter records who was first.
package test {
	private import ScalarValues::*;

	action monitor {
		attribute temp : Integer = 10;
		attribute order : String = "";
		first start;
		fork split;
		action heat { assign temp := 40; assign order := order + "h"; }
		action awaitWarm accept when temp > 20;
		action alert { assign order := order + "a"; }
		action idle { assign order := order + "i"; }
		join sync;
		done;
		succession first start then split;
		succession first split then heat;
		succession first split then awaitWarm;
		succession first split then idle;
		succession first awaitWarm then alert;
		succession first heat then sync;
		succession first alert then sync;
		succession first idle then sync;
		succession first sync then done;
	}
}
