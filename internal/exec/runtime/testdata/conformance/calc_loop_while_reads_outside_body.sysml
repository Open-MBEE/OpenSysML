// A loop's `while` is beside its body: on every iteration it reads the package's
// `limit`, not the one the body declared the iteration before.
package test {
	private import ScalarValues::*;
	attribute limit : Integer = 3;
	calc counted {
		attribute i : Integer = 0;
		while i < limit {
			assign i := i + 1;
			attribute limit : Integer = 0;
		}
		return : Integer = i;
	}
}
