// A bare `in` parameter subsetting two features holds the intersection of
// their ranges (KerML 1.0 §7.3.4.4): [0..1] and [1..*] bound it to [1..1], so
// invoking the calc without an argument fails as unbound.
package test {
	private import ScalarValues::*;
	attribute f1 : Real[0..1];
	attribute f2 : Real[1..*];
	calc def Score { in x : Real subsets f1, f2; return : Real = x; }
}
