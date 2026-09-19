package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Subsystem;
	part def OpticalSubsystem :> Subsystem;
	part def MirrorAssembly :> OpticalSubsystem;

	requirement massRequirement;
	requirement pointingRequirement;
	requirement dataRequirement;

	part telescope : MirrorAssembly {
		satisfy massRequirement;
	}
	part mount : Subsystem {
		satisfy pointingRequirement;
	}
	part groundStation {
		satisfy massRequirement;
	}

	verification def MassTest;
	verification massVerification : MassTest {
		objective {
			verify massRequirement;
		}
	}
	verification pointingVerification : MassTest {
		objective {
			verify pointingRequirement;
			verify massRequirement;
		}
	}

	calc def Matrix :> Query {
		in root : Element;
		Project(
			source = WhereType(
				source = Descendants(source = root, maxDepth = 1),
				type = "RequirementUsage"
			),
			properties = ("name"),
			columns = (
				RelatedColumn("satisfiedBy", "satisfaction", "incoming", 1),
				RelatedColumn("verifiedBy", "verification", "incoming", 1),
				RelatedColumn("verifications", "verification", "incoming", 1, "count"),
				RelatedColumn("verified", "verification", "incoming", 1, "any")
			)
		)
	}

	calc def Traced :> Query {
		in root : Element;
		in kind : String;
		in direction : String;
		in maxDepth : Integer = 1;
		in aggregate : String = "list";
		Project(
			source = root,
			properties = ("name"),
			columns = (
				RelatedColumn(
					name = "related",
					relationshipKind = kind,
					direction = direction,
					maxDepth = maxDepth,
					aggregate = aggregate
				)
			)
		)
	}

	calc def Uncovered :> Query {
		in root : Element;
		WhereFeature(
			source = Matrix(root = root),
			'feature' = "verifications",
			operator = "=",
			value = "0"
		)
	}

	calc def Unverified :> Query {
		in root : Element;
		WhereFeature(
			source = Matrix(root = root),
			'feature' = "verified",
			operator = "=",
			value = "false"
		)
	}

	calc def MostVerified :> Query {
		in root : Element;
		OrderBy(
			source = Matrix(root = root),
			property = "verifications",
			direction = "descending",
			missing = "last",
			multiple = "error"
		)
	}

	calc def SatisfiedBy :> Query {
		in root : Element;
		in operator : String;
		in satisfier : String;
		WhereFeature(
			source = Matrix(root = root),
			'feature' = "satisfiedBy",
			operator = operator,
			value = satisfier
		)
	}

	calc def BySatisfier :> Query {
		in root : Element;
		in multiple : String;
		OrderBy(
			source = Matrix(root = root),
			property = "satisfiedBy",
			direction = "ascending",
			missing = "last",
			multiple = multiple
		)
	}
}
