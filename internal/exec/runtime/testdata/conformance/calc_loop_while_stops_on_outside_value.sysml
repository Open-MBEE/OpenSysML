// A loop's `while` reads the package's `limit` of 1 and stops after one iteration:
// the larger `limit` its body declares does not keep the loop running.
package test {
	private import ScalarValues::*;
	attribute limit : Integer = 1;
	calc counted {
		attribute i : Integer = 0;
		while i < limit {
			assign i := i + 1;
			attribute limit : Integer = 7;
		}
		return : Integer = i;
	}
}
