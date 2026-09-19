// The index and the quantity share one syntax node, distinguished by the
// bracket they are written with: `#( i )` indexes a sequence and `[ unit ]`
// gives a magnitude its unit. This case writes both in one body, so neither can
// be read as the other without failing.
package test {
	private import ScalarValues::*;
	private import SI::*;

	calc secondHeight {
		attribute heights : Real[*] = (1.0, 2.5, 4.0);
		return : Real = heights#(2) [m] / 1.0 [m];
	}
}
