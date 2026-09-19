// The simplest traceability report: three flat requirements, three parts that
// satisfy two of them, one table saying which part covers each requirement
// and a list of the requirements nothing covers. No verification, no
// hierarchy, no derivation — the shape every larger report in this series
// builds on.
package RoverBasic {
	private import DocumentQueries::*;
	private import KerML::Root::Element;

	// ---- Requirements ----

	part specification {
		requirement <'RV-1'> range {
			doc /* The rover shall travel at least 20 km on one charge. */
		}
		requirement <'RV-2'> payload {
			doc /* The rover shall carry a 15 kg science payload. */
		}
		requirement <'RV-3'> uplink {
			doc /* The rover shall accept commands from the lander at 2 kbps. */
		}
	}

	// ---- Design ----

	part def Battery;
	part def Chassis;
	part def Radio;

	part rover {
		part battery : Battery;
		part chassis : Chassis;
		part radio : Radio;

		satisfy specification.range by battery;
		satisfy specification.payload by chassis;
	}

	// ---- Queries ----

	// Every requirement declared directly under the root.
	calc def Requirements :> Query {
		in root : Element;
		WhereType(source = OwnedElements(source = root), type = "RequirementUsage")
	}

	// Each requirement with the parts that satisfy it and whether any does.
	calc def SatisfactionMatrix :> Query {
		in root : Element;
		Project(
			source = Requirements(root = root),
			properties = ("shortName", "documentation"),
			columns = (
				RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "satisfied", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1, aggregate = "any")
			)
		)
	}

	// The requirements no part satisfies.
	calc def Unsatisfied :> Query {
		in root : Element;
		Project(
			source = WhereRelated(
				source = Requirements(root = root),
				relationshipKind = "satisfaction",
				direction = "incoming",
				maxDepth = 1,
				exists = false
			),
			properties = ("shortName")
		)
	}

	// ---- Document ----

	part def BasicReport :> Document {
		attribute redefines title = "Rover Requirement Satisfaction";

		part intro : Paragraph {
			attribute redefines text = "Which part of the rover satisfies each requirement.";
		}

		part matrix : Table {
			attribute redefines caption = "Requirements and their satisfiers";
			calc rows : SatisfactionMatrix {
				in root = specification;
			}
		}

		part gaps : Paragraph {
			attribute redefines text = "Requirements no part satisfies:";
		}

		part gapList : List {
			calc items : Unsatisfied {
				in root = specification;
			}
		}
	}
}
