// A loop's `until` is beside its body: it reads the package's `limit`, not the
// one the body declares, so the loop runs three times.
package test {
	private import ScalarValues::*;
	attribute limit : Integer = 3;
	calc counted {
		attribute i : Integer = 0;
		loop {
			assign i := i + 1;
			attribute limit : Integer = 0;
		} until i >= limit;
		return : Integer = i;
	}
}
