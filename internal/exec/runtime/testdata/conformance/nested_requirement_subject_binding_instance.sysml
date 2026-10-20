// An object of a definition owning requirement usages carries each as a nested
// object whose subject holds what the usage binds it to from the owning object,
// and whose `result` is the requirement's verdict on that object.
package test {
	private import ScalarValues::*;

	requirement def R1 { subject s : Integer; require constraint { s > 0 } }
	requirement def RP { subject s : PA; require constraint { s.v > 0 } }
	part def PA { attribute v : Integer = 5; }

	part def P1 {
		attribute sub : Integer = 3;
		attribute neg : Integer = -2;
		part pa : PA;
		requirement rq : R1 { subject s :>> s = sub; }
		requirement rqBare : R1 { subject = sub; }
		requirement rqPart : RP { subject s :>> s = pa; }
		requirement rqFail : R1 { subject s :>> s = neg; }
		requirement rqUnbound : R1 { subject s :>> s; }
		satisfy requirement sat : RP by pa;
		not satisfy requirement nsat : RP by pa;
		use case uq : U1;
	}
	use case def U1 { subject s : PA; }
}
