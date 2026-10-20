// A multi-valued unvalued `out results : Boolean[2]` of an exhibited state
// definition stands in with one object per required value; the entry action's
// assignment replaces them all, and a held image carries the graph the run leaves.
package test {
	private import ScalarValues::*;
	state def Life {
		out results : Boolean[2];
		entry; then work;
		state work {
			entry action e { assign results := (true, false); }
		}
	}
	part def Host { exhibit state life : Life; }
	part host : Host;
}
