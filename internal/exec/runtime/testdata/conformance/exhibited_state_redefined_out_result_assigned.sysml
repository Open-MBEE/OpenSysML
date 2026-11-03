// An unvalued `out result` redefined as `out res :>> result` is one feature value
// under both names; the entry action's assignment under the redefining name
// replaces the stand-in, and the machine's data agrees under either name.
package test {
	private import ScalarValues::*;
	state def Base { out result : Boolean[1]; }
	state def Life :> Base {
		out res :>> result;
		entry; then work;
		state work {
			entry action e { assign res := true; }
		}
	}
	part def Host { exhibit state life : Life; }
	part host : Host;
}
