// Requirement identifiers and doc text as document prose: the model gives each
// requirement a short name and a doc comment; the Reqs query projects both,
// and the report renders them once as a table and once as definitions.
// The Tree query lists every requirement definition and usage under a root in
// hierarchy order, which the report renders as a third table.
package Requirements {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	requirement def <'HLR-R001'> CrewSafety {
		doc /* The mission shall safely return all three crew members to Earth. */
	}
	requirement def <'HLR-R002'> SoftLanding {
		doc /* The mission shall achieve a soft landing
		     * on the lunar surface. */
	}
	part spec {
		requirement <'R1'> r1 : CrewSafety {
			doc /* Applies to every phase of the mission. */
		}
		requirement <'R2'> r2 : SoftLanding {
			doc /* Applies to the powered descent. */
			requirement <'R2.1'> descentRate {
				doc /* The vertical velocity at touchdown shall not exceed 2 m/s. */
			}
			requirement <'R2.2'> tiltAngle {
				doc /* The lander shall come to rest within 12 degrees of vertical. */
			}
		}
	}

	calc def Reqs :> Query {
		in root : Element;
		Project(
			source = RelatedElements(
				source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "RequirementUsage"),
				relationshipKind = "typing", direction = "outgoing", maxDepth = 1
			),
			properties = ("shortName", "name", "documentation")
		)
	}

	calc def Tree :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = Union(
					source = WhereType(source = Descendants(source = root, maxDepth = 3), type = "RequirementDefinition"),
					other = WhereType(source = Descendants(source = root, maxDepth = 3), type = "RequirementUsage")
				),
				property = "qualifiedName",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("shortName", "name", "documentation")
		)
	}

	part def RequirementsReport :> Document {
		attribute redefines title = "Mission Requirements";

		part intro : Paragraph {
			attribute redefines text = "Each requirement by identifier, first as a table and then as prose; then the specification's requirement hierarchy.";
		}

		part table : Table {
			attribute redefines caption = "Requirements";
			calc rows : Reqs {
				in root = spec;
			}
		}

		part prose : Definitions {
			attribute redefines term = "shortName";
			attribute redefines description = "documentation";
			calc rows : Reqs {
				in root = spec;
			}
		}

		part tree : Table {
			attribute redefines caption = "Requirement hierarchy";
			calc rows : Tree {
				in root = spec;
			}
		}
	}
}
