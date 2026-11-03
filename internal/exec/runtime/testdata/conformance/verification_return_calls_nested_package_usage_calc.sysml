// A verification's returned expression calls a calc of an object nested in a package-level
// requirement usage no object stands for (`req.inner.scale(3.0)`): the receiver is reached
// and the calc applied over it under the one probe that reads the usage.
package test {
	private import ScalarValues::Real;
	private import VerificationCases::VerdictKind;

	part def Holder {
		attribute factor : Real = 2.0;
		calc scale { in x : Real; return : Real = x * factor; }
	}
	requirement def Sized {
		attribute limit : Real;
		part inner : Holder;
	}
	requirement req : Sized {
		attribute redefines limit = 6.0;
	}

	verification check {
		objective { verify req; }
		return verdict : VerdictKind =
			if req.inner.scale(3.0) == req.limit ? VerdictKind::pass else VerdictKind::fail;
	}
}
