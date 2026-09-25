// A qualified name several members answer to is the ambiguous reference the
// checker reports, with its candidate count, rather than an unresolved one.
package Twice {
	private import ScalarValues::*;
	attribute t : Integer = 1;
	attribute t : Integer = 2;
}
package test {
	private import ScalarValues::*;

	calc probe {
		return : Integer = Twice::t;
	}
}
