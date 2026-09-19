// A private import surfaces a name only inside the importing package, so a
// qualified name reaching for it from outside is the unresolved reference the
// checker reports, not a member read.
package A {
	private import ScalarValues::*;
	attribute x : Integer = 1;
}
package Priv { private import A::*; }
package test {
	private import ScalarValues::*;

	calc probe {
		return : Integer = Priv::x;
	}
}
