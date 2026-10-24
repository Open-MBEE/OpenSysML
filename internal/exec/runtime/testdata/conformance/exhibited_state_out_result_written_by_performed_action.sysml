// A performed action of the part writes the exhibited state's unvalued `out result`
// from outside the machine: the machine's data holds the written value, the
// stand-in goes, and a held image carries the graph the run leaves.
package test {
	private import ScalarValues::*;
	state def Life {
		out result : Boolean[1];
		entry; then idle;
		state idle;
	}
	part def Host {
		exhibit state life : Life;
		perform action set { assign life.result := true; }
	}
	part host : Host;
}
