// A subject, assume or require constraint declaration takes an identification
// like any usage (SysML.xtext SubjectUsage and RequirementConstraintUsage both
// reach UsageDeclaration → Identification), so `<short>` is allowed with or
// without a name, with prefix metadata, typing, multiplicity, a value and a body.
package RequirementMemberShortNames {
	metadata def M;
	part def T;
	constraint def C;

	requirement def R1 {
		subject <s> x : T;
		assume constraint <a> ac : C;
		require constraint <r> rc : C;
	}

	requirement def R2 {
		subject <s> : T;
		assume constraint <a> : C;
		require constraint <r> : C;
	}

	requirement def R3 {
		subject #M <s> x : T[1] :>> R1::s default R1::s {
			doc /* the subject */
		}
		assume #M constraint <a> ac : C[1] = true {
			doc /* the assumption */
		}
		require #M constraint <r> rc : C[1] = true {
			x != null
		}
	}

	requirement def R5 :> R1 {
		subject :>> s;
		assume constraint :>> a;
		require constraint <r2> :>> r;
	}

	requirement def R4 {
		subject <s> = R1::s;
		assume constraint <a> { true }
		require constraint <r> { true }
	}
}
