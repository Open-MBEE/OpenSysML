package test {
	private import ScalarValues::*;
	private import VerificationCases::*;

	verification def OrderCheck {
		return verdict : VerdictKind;
		attribute x : Integer := 1;
		objective order {
			require constraint { x == 12 }
		}
		action s1 { assign x := x * 10; }
		action s2 { assign x := x + 2; }
		VerificationCases::PassIf(x == 12)
	}
}
