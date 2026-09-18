// A subject, actor, stakeholder, objective, variant, assume or require member
// takes its prefix metadata after the keyword (SysML.xtext SubjectUsage,
// ActorUsage, StakeholderUsage, ObjectiveRequirementUsage, VariantUsageElement,
// RequirementConstraintUsage: `'keyword' UsageExtensionKeyword* …`), while an
// assertion takes it ahead of `assert` (AssertConstraintUsage,
// `OccurrenceUsagePrefix 'assert'`). Every form here is the accepted placement.
package MemberPrefixMetadataPlacement {
	metadata def M;
	metadata def B;
	part def T;
	part def A;
	constraint def C;
	requirement def R;

	requirement def Req {
		subject #M s : T;
		actor #M a : A;
		stakeholder #M k;
		assume #M constraint a : C;
		require #M constraint r : C;
		require #M <r2> rc : C;
	}

	requirement def ReqShort {
		subject #M <s> x : T;
		assume #M constraint <a> ac : C;
		require #M <r> rc : C;
	}

	use case def U {
		objective #M o : R;
	}

	variation part def V {
		variant #M part v;
	}

	part def D {
		#B assert not constraint c;
	}
}
