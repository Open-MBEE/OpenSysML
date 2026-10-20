// A state definition a part exhibits with an unvalued `out result : Boolean[1]`
// (KerML 1.0 §7.4.7.2: a parameter is a feature of the behavior; SysML v2 §7.17.2,
// §7.18.2: a state definition's parameters are those of its performances): the
// unvalued output stays unvalued, the machine runs, and its data reads by name what
// the action it calls wrote to the machine's occurrence.
package test {
	private import ScalarValues::*;
	action def Observer {
		in ref context : Life[1];
		first o;
		action o { assign context.flag := true; }
		first o then done;
	}
	state def Life {
		out result : Boolean[1];
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
