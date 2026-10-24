// A state machine a part exhibits reads by name what an action it calls wrote
// to its occurrence: the call takes the machine (`context = life`), writes
// `context.flag`, and the entry action's `assign seen := flag` sees the write.
package test {
	private import ScalarValues::*;
	action def Observer {
		in ref context : Life[1];
		first o;
		action o { assign context.flag := true; }
		first o then done;
	}
	state def Life {
		attribute flag : Boolean = false;
		attribute seen : Boolean = false;
	}
	part def Host {
		exhibit state life : Life {
			entry; then work;
			state work {
				entry action run {
					first call;
					action call : Observer { in ref :>> context = life; }
					first call then note;
					action note { assign seen := flag; }
				}
			}
		}
	}
	part host : Host;
}
