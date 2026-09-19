library package OpenSysMLMathFunctions {
	doc
	/*
	 * NON-NORMATIVE. This package is an OpenSysML extension, not part of the OMG KerML
	 * standard library: it declares the exponential, logarithmic and two-argument
	 * arctangent functions the Kernel Function Library omits. The vendored OMG files are
	 * unmodified. A model that needs these writes `import OpenSysMLMathFunctions::*;`.
	 */

	public import ScalarValues::*;

	function exp {
		doc
		/*
		 * The exponential of x: e raised to the power x.
		 */
		in x: Real[1]; return : Real[1];
	}

	function ln {
		doc
		/*
		 * The natural logarithm of x, the inverse of exp. Defined for x > 0.0.
		 */
		in x: Real[1]; return : Real[1];
	}

	function log {
		doc
		/*
		 * The logarithm of x to the given base, ln(x) / ln(base). Defined for x > 0.0 and
		 * a base that is positive and not 1.0. The base is a parameter rather than
		 * implied, so a call states whether it means base 10, base 2 or another base.
		 */
		in x: Real[1]; in base: Real[1]; return : Real[1];
	}

	function atan2 {
		doc
		/*
		 * The angle in radians from the positive x axis to the point (x, y), in
		 * (-pi, pi]. The parameters are ordered y then x, as in IEEE 754, so the angle
		 * carries the quadrant that arctan(y / x) loses. Undefined at the origin.
		 */
		in y: Real[1]; in x: Real[1]; return : Real[1];
	}
}
