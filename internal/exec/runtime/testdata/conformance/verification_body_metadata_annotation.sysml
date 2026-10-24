// A `@VerificationMethod { … }` written among a verification definition's
// body members annotates the case (SysML v2 §7.26 Metadata): it is no step of
// the body, so the result expression runs as it does with the annotation on
// the usage or stated from outside with `metadata … about`.
package test {
	private import ScalarValues::Real;
	private import VerificationCases::*;

	part def Thing { attribute v : Real; }
	part thing : Thing { attribute :>> v = 3.0; }
	requirement big { subject t : Thing; require constraint { t.v >= 1.0 } }

	verification def Checked {
		subject t : Thing;
		objective check { verify big; }
		@VerificationMethod { kind = VerificationMethodKind::test; }
		PassIf(t.v >= 1.0)
	}
	verification checked : Checked { subject t = thing; }
}
