// A model the run gate evaluates against: `sysml model.sysml -e total` answers 6.0,
// which needs the file read, the embedded library loaded and the name resolved —
// nothing about it is in-process-only.
package gatedemo {
	private import ScalarValues::*;

	part def Widget {
		attribute mass : Real = 3.0;
	}

	attribute total : Real = 6.0;
}
