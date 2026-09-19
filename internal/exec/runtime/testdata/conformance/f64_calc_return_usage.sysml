// F64: a `return` may declare a usage (SysML.xtext:1961 `ReturnParameterMember`),
// so the returned feature carries its own name, type and value expression.
package test {
	private import ScalarValues::*;

	calc scaled {
		in x : Real;
		return attribute doubled : Real = x * 2.0;
	}
}
