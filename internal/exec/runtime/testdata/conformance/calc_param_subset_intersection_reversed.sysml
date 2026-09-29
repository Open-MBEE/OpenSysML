// The same intersection in the other written order (KerML 1.0 §7.3.4.4):
// [1..*] and [0..1] bound the parameter to [1..1] likewise, so the unbound
// invocation fails the same way.
package test {
	private import ScalarValues::*;
	attribute f2 : Real[1..*];
	attribute f1 : Real[0..1];
	calc def Score { in x : Real subsets f2, f1; return : Real = x; }
}
