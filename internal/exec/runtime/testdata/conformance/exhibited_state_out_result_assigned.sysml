// An unvalued `out result` of a state definition a part exhibits stands in for
// its value until the entry action assigns one; the stand-in then goes, so the
// object graph a held image carries is the graph the run leaves.
package test {
	private import ScalarValues::*;
	state def Life {
		out result : Boolean[1];
		entry; then work;
		state work {
			entry action e { assign result := true; }
		}
	}
	part def Host { exhibit state life : Life; }
	part host : Host;
}
