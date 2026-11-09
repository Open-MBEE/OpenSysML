// A calc def's declared return redefines the Evaluation::result it reaches
// through Calculation (KerML 8.4.4.6), so the library result is not among its
// inherited members (KerML 8.3.3.1): the members are x and r alone.
package P {
	private import ScalarValues::*;
	calc def Calc { in x : Real; return r : Real; }
}
