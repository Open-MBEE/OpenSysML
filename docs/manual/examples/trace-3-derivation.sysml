// Traceability along derivation and refinement: mission requirements derive
// system requirements, which derive subsystem requirements, three levels of
// a specification tree apart. Design elements refine the leaves, parts satisfy
// some of them, and the report follows the chains in both directions, at one
// hop and to their ends.
package RangeDerivation {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	// ---- Design vocabulary ----

	part def Battery {
		attribute capacity : Real;
	}
	part def DriveMotor {
		attribute efficiency : Real;
	}
	part def HeaterLoop;
	analysis def EnergyBudgetAnalysis;

	// ---- The specification: three levels of requirements ----

	part specification {
		part mission {
			requirement <'M-1'> range {
				doc /* The rover shall traverse 20 km over the mission. */
			}
			requirement <'M-2'> lifetime {
				doc /* The rover shall operate for 90 sols. */
			}
		}
		part system {
			requirement <'S-1'> dailyRange {
				doc /* The rover shall drive 500 m per sol. */
			}
			requirement <'S-2'> energyBudget {
				doc /* The rover shall store 1.2 kWh for each sol's drive. */
			}
			requirement <'S-3'> nightSurvival {
				doc /* The rover shall survive nights at -90 C. */
			}
		}
		part subsystem {
			requirement <'B-1'> batteryCapacity {
				doc /* The battery shall hold 1.5 kWh. */
			}
			requirement <'B-2'> batteryHeater {
				doc /* The battery shall be kept above -20 C. */
			}
			requirement <'D-1'> driveEfficiency {
				doc /* The drive shall convert 85 percent of electrical energy to motion. */
			}
		}
	}

	// ---- Derivation: each system requirement from a mission one, each
	// subsystem requirement from a system one, in either notation ----

	#RequirementDerivation::derivation connection deriveDailyRange
		connect specification.mission.range to specification.system.dailyRange;
	#RequirementDerivation::derivation connection deriveEnergyBudget
		connect specification.mission.range to specification.system.energyBudget;
	#RequirementDerivation::derivation connection deriveNightSurvival
		connect specification.mission.lifetime to specification.system.nightSurvival;
	connection deriveBatteryCapacity : RequirementDerivation::Derivation
		connect specification.system.energyBudget to specification.subsystem.batteryCapacity;
	connection deriveDriveEfficiency : RequirementDerivation::Derivation
		connect specification.system.energyBudget to specification.subsystem.driveEfficiency;
	connection deriveBatteryHeater : RequirementDerivation::Derivation
		connect specification.system.nightSurvival to specification.subsystem.batteryHeater;

	// ---- Refinement: an analysis sharpens the energy budget, a design
	// element makes the heater requirement concrete ----

	#ModelingMetadata::refinement dependency analysisRefinesBudget
		from EnergyBudgetAnalysis to specification::system::energyBudget;
	#ModelingMetadata::refinement dependency heaterRefinesRequirement
		from HeaterLoop to specification::subsystem::batteryHeater;

	// ---- The design: two leaves satisfied, two not ----

	part rover {
		part battery : Battery {
			attribute :>> capacity = 1.5;
		}
		part drive : DriveMotor {
			attribute :>> efficiency = 0.87;
		}
		part heater : HeaterLoop;
		satisfy specification.subsystem.batteryCapacity by battery;
		satisfy specification.subsystem.driveEfficiency by drive;
	}

	// ---- Queries ----

	// Descendants visits level by level, so the requirements come out
	// mission first, then system, then subsystem, each in declaration order.
	calc def Requirements :> Query {
		in root : Element;
		WhereType(source = Descendants(source = root, maxDepth = 2), type = "RequirementUsage")
	}

	calc def Identified :> Query {
		in rows : Element[0..*] ordered;
		Project(source = rows, properties = ("shortName", "name", "documentation"))
	}

	calc def DirectlyDerived :> Query {
		in req : Element;
		Identified(rows = RelatedElements(source = req, relationshipKind = "derivation", direction = "outgoing", maxDepth = 1))
	}

	// Everything that ultimately derives from req, however many hops away.
	calc def AllDerived :> Query {
		in req : Element;
		Identified(rows = RelatedElements(source = req, relationshipKind = "derivation", direction = "outgoing", maxDepth = 3))
	}

	// The chain back from req to the mission requirement it descends from.
	calc def Origins :> Query {
		in req : Element;
		Identified(rows = RelatedElements(source = req, relationshipKind = "derivation", direction = "incoming", maxDepth = 3))
	}

	calc def Lineage :> Query {
		in root : Element;
		Project(
			source = Requirements(root = root),
			properties = ("shortName", "name"),
			columns = (
				RelatedColumn(name = "derivedFrom", relationshipKind = "derivation", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "derives", relationshipKind = "derivation", direction = "outgoing", maxDepth = 1),
				RelatedColumn(name = "descendants", relationshipKind = "derivation", direction = "outgoing", maxDepth = 3, aggregate = "count"),
				RelatedColumn(name = "refinedBy", relationshipKind = "refinement", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1)
			)
		)
	}

	// Requirements nothing derives: the top of every chain.
	calc def Originals :> Query {
		in root : Element;
		Identified(rows = WhereRelated(
			source = Requirements(root = root),
			relationshipKind = "derivation", direction = "incoming", maxDepth = 1, exists = false
		))
	}

	// Requirements deriving nothing further: the leaves a design must satisfy.
	calc def Leaves :> Query {
		in root : Element;
		WhereRelated(
			source = Requirements(root = root),
			relationshipKind = "derivation", direction = "outgoing", maxDepth = 1, exists = false
		)
	}

	// Leaves no part satisfies: where the chain of derivation ends in nothing.
	calc def UnsatisfiedLeaves :> Query {
		in root : Element;
		Identified(rows = WhereRelated(
			source = Leaves(root = root),
			relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1, exists = false
		))
	}

	// ---- Document ----

	part def DerivationReport :> Document {
		attribute redefines title = "Rover Requirement Derivation";

		part intro : Paragraph {
			attribute redefines text = "How the mission requirements flow down to the subsystems, what refines them, and where the flow-down ends without a part to satisfy it.";
		}

		part lineage : Section {
			attribute redefines title = "Lineage";
			part table : Table {
				attribute redefines caption = "Every requirement with its parent, children, descendant count, refinements and satisfiers";
				calc rows : Lineage {
					in root = specification;
				}
			}
		}

		part chains : Section {
			attribute redefines title = "Following a chain";
			part down : Table {
				attribute redefines caption = "Everything derived from M-1, at any depth";
				calc rows : AllDerived {
					in req = specification.mission.range;
				}
			}
			part direct : Table {
				attribute redefines caption = "Derived directly from M-1";
				calc rows : DirectlyDerived {
					in req = specification.mission.range;
				}
			}
			part up : Table {
				attribute redefines caption = "The chain above B-1";
				calc rows : Origins {
					in req = specification.subsystem.batteryCapacity;
				}
			}
		}

		part ends : Section {
			attribute redefines title = "Chain ends";
			part tops : Table {
				attribute redefines caption = "Requirements at the top of a chain";
				calc rows : Originals {
					in root = specification;
				}
			}
			part gaps : Table {
				attribute redefines caption = "Leaves no part satisfies";
				calc rows : UnsatisfiedLeaves {
					in root = specification;
				}
			}
		}
	}
}
