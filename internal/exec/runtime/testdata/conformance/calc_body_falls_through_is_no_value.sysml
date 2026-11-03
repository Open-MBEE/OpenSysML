// A body that runs to its end on a path with no `return` leaves the result
// parameter unbound, which is reported as no value, exactly as an output the
// body never assigns is (KerML 7.4.8.2, SysML 7.19.2 say only how a result is bound).
package test {
	private import ScalarValues::*;

	calc maybe {
		in n : Integer;
		if n < 0 {
			return : Integer = n;
		}
	}
}
