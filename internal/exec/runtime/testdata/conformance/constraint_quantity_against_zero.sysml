// A constraint comparing quantities with a bare zero, as the geometry library
// writes `xoffset > 0` over a LengthValue: zero is read in the quantity's unit.
package test {
	private import ISQ::*;
	private import SI::*;

	attribute xoffset : LengthValue = 3.0 [m];
	attribute yoffset : LengthValue = 0 [mm];

	constraint offsetsAreNonNegative {
		xoffset > 0 and yoffset >= 0 and yoffset == 0 and 0 < xoffset and not (xoffset < 0)
	}
}
