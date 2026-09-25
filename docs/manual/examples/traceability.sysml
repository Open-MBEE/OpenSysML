// Requirement traceability in one document: the requirements as a list, a
// matrix with every requirement's satisfiers, verifiers and verification
// count on one row, and the verdicts of checking the spacecraft against them.
package Traceability {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	// ---- The system model the report is generated from ----

	part def Bus {
		attribute dryMass : Real;
	}
	part def Antenna {
		attribute gain : Real;
	}
	part def Radiator {
		attribute area : Real;
	}

	requirement def MassLimit {
		subject bus : Bus;
		require constraint { bus.dryMass <= 1200.0 }
	}
	requirement def DownlinkGain {
		subject antenna : Antenna;
		require constraint { antenna.gain >= 40.0 }
	}
	requirement def HeatRejection {
		subject radiator : Radiator;
		require constraint { radiator.area >= 2.0 }
	}
	requirement def Passivation;

	part specification {
		requirement <'SC-1'> massLimit : MassLimit {
			doc /* The bus dry mass shall not exceed 1200 kg. */
		}
		requirement <'SC-2'> downlinkGain : DownlinkGain {
			doc /* The high-gain antenna shall provide at least 40 dBi. */
		}
		requirement <'SC-3'> heatRejection : HeatRejection {
			doc /* The radiator shall provide at least 2 m² of rejection area. */
		}
		requirement <'SC-4'> passivation : Passivation {
			doc /* The spacecraft shall be passivated at end of mission. */
		}
	}

	part spacecraft {
		part bus : Bus {
			attribute :>> dryMass = 1150.0;
		}
		part antenna : Antenna {
			attribute :>> gain = 42.5;
		}
		part radiator : Radiator {
			attribute :>> area = 1.6;
		}
		satisfy specification.massLimit by bus;
		satisfy specification.downlinkGain by antenna;
		satisfy specification.heatRejection by radiator;
	}

	verification def MassMeasurement {
		subject bus : Bus;
		VerificationCases::PassIf(bus.dryMass <= 1200.0)
	}
	verification massMeasurement : MassMeasurement {
		subject bus = spacecraft.bus;
		objective {
			verify specification.massLimit;
		}
	}

	verification def GainTest {
		subject antenna : Antenna;
		VerificationCases::PassIf(antenna.gain >= 40.0)
	}
	verification gainTest : GainTest {
		subject antenna = spacecraft.antenna;
		objective {
			verify specification.downlinkGain;
		}
	}
	verification gainAnalysis : GainTest {
		subject antenna = spacecraft.antenna;
		objective {
			verify specification.downlinkGain;
		}
	}

	// ---- Document queries ----

	calc def Requirements :> Query {
		in root : Element;
		Project(
			source = WhereType(
				source = Descendants(source = root, maxDepth = 1),
				type = "RequirementUsage"
			),
			properties = ("shortName", "name", "documentation")
		)
	}

	calc def TraceMatrix :> Query {
		in root : Element;
		Project(
			source = WhereType(
				source = Descendants(source = root, maxDepth = 1),
				type = "RequirementUsage"
			),
			properties = ("shortName", "name"),
			columns = (
				RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "verifiedBy", relationshipKind = "verification", direction = "incoming", maxDepth = 1),
				RelatedColumn(
					name = "verifications",
					relationshipKind = "verification",
					direction = "incoming",
					maxDepth = 1,
					aggregate = "count"
				)
			)
		)
	}

	calc def Checks :> Query {
		in root : Element;
		Project(
			source = Verdicts(source = root),
			properties = ("kind", "name", "path", "verdict", "condition")
		)
	}

	// ---- The document definition ----

	part def TraceabilityReport :> Document {
		attribute redefines title = "Spacecraft Requirement Traceability";

		part intro : Paragraph {
			attribute redefines text = "Every requirement, what satisfies and verifies it, and whether the spacecraft meets it.";
		}

		part requirements : Section {
			attribute redefines title = "Requirements";

			part list : Definitions {
				attribute redefines term = "shortName";
				attribute redefines description = "documentation";
				calc rows : Requirements {
					in root = specification;
				}
			}
		}

		part matrix : Section {
			attribute redefines title = "Traceability Matrix";

			part table : Table {
				attribute redefines caption = "Requirements with their satisfiers and verifiers";
				calc rows : TraceMatrix {
					in root = specification;
				}
			}
		}

		part verdicts : Section {
			attribute redefines title = "Verdicts";

			part table : Table {
				attribute redefines caption = "Checks of the spacecraft against its requirements";
				calc rows : Checks {
					in root = spacecraft;
				}
			}
		}
	}
}
