// A result parameter stating no multiplicity is held in its effective `[0..*]` (SysML v2 §7.6.3, §7.19.2): one value is a one-element sequence, while a body local whose multiplicity is omitted keeps the implicit `[1..1]` of an attribute usage.
package test {
	private import ScalarValues::*;

	calc def Shaped {
		in x : Real;
		return : Real = x;
	}
}
