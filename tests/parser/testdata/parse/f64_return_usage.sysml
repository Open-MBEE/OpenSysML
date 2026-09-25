// F64: `ReturnParameterMember` is `'return' UsageElement` (SysML.xtext:1961), so
// a return may declare a usage rather than state a result expression.
package ReturnUsages {
	private import ScalarValues::*;
	part def Engine;
	calc def SelectEngine {
		in engine : Engine[*];
		return selectedEngine :> engine;
	}
	calc def Profile {
		return attribute accelerationProfile : Real[*] := ();
	}
	calc def Sum {
		in a : Real;
		in b : Real;
		a + b
	}
}
