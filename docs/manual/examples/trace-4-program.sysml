// Program-level traceability across packages: stakeholder needs derive
// system requirements, which derive subsystem requirements owned by three
// teams, one of which keeps a derived requirement of its own inside its
// design package. Each team's design satisfies, refines and verifies its
// requirements; the report joins everything into one matrix grouped by owner,
// then combines and subtracts result sets to find the gaps that matter.

package OrbiterVocabulary {
	private import ScalarValues::*;

	part def SolarArray {
		attribute endOfLifePower : Real;
	}
	part def Battery {
		attribute depthOfDischarge : Real;
	}
	part def Transmitter {
		attribute dataRate : Real;
		attribute linkMargin : Real;
	}
	part def HeaterBank;
}

package ProgramRequirements {
	private import ScalarValues::*;
	private import OrbiterVocabulary::*;

	// Every requirement carries the team that owns it and how critical it is.
	requirement def ProgramRequirement {
		attribute team : String;
		attribute priority : String;
	}
	requirement def ArrayOutput :> ProgramRequirement {
		subject array : SolarArray;
		require constraint { array.endOfLifePower >= 1100.0 }
	}
	requirement def BatteryDepth :> ProgramRequirement {
		subject battery : Battery;
		require constraint { battery.depthOfDischarge <= 0.4 }
	}
	requirement def LinkMargin :> ProgramRequirement {
		subject transmitter : Transmitter;
		require constraint { transmitter.linkMargin >= 3.0 }
	}
	requirement def DataRate :> ProgramRequirement {
		subject transmitter : Transmitter;
		require constraint { transmitter.dataRate >= 4.0 }
	}

	part specification {
		part needs {
			requirement <'ST-1'> science : ProgramRequirement {
				doc /* The orbiter shall return 5 TB of science data per year. */
				attribute :>> team = "Program";
				attribute :>> priority = "critical";
			}
			requirement <'ST-2'> longevity : ProgramRequirement {
				doc /* The orbiter shall operate for 4 years. */
				attribute :>> team = "Program";
				attribute :>> priority = "critical";
			}
		}
		part system {
			requirement <'SYS-1'> downlink : ProgramRequirement {
				doc /* The orbiter shall downlink 14 GB per orbit. */
				attribute :>> team = "Systems";
				attribute :>> priority = "critical";
			}
			requirement <'SYS-2'> power : ProgramRequirement {
				doc /* The orbiter shall generate 900 W at end of life. */
				attribute :>> team = "Systems";
				attribute :>> priority = "critical";
			}
			requirement <'SYS-3'> survival : ProgramRequirement {
				doc /* The orbiter shall keep every unit within its survival temperatures through eclipse. */
				attribute :>> team = "Systems";
				attribute :>> priority = "high";
			}
			requirement <'SYS-4'> telemetry : ProgramRequirement {
				doc /* The orbiter shall report housekeeping telemetry once per minute. */
				attribute :>> team = "Systems";
				attribute :>> priority = "low";
			}
		}
		part subsystem {
			requirement <'PWR-1'> arrayOutput : ArrayOutput {
				doc /* The solar array shall deliver 1100 W at end of life. */
				attribute :>> team = "Power";
				attribute :>> priority = "critical";
			}
			requirement <'PWR-2'> batteryDepth : BatteryDepth {
				doc /* The battery shall not discharge below 40 percent in eclipse. */
				attribute :>> team = "Power";
				attribute :>> priority = "high";
			}
			requirement <'COM-1'> linkMargin : LinkMargin {
				doc /* The X-band link shall close with 3 dB of margin at maximum range. */
				attribute :>> team = "Comms";
				attribute :>> priority = "critical";
			}
			requirement <'COM-2'> dataRate : DataRate {
				doc /* The transmitter shall sustain 4 Mbps. */
				attribute :>> team = "Comms";
				attribute :>> priority = "critical";
			}
			requirement <'THM-1'> heaterPower : ProgramRequirement {
				doc /* Survival heaters shall draw no more than 60 W. */
				attribute :>> team = "Thermal";
				attribute :>> priority = "critical";
			}
		}
	}

	// ---- Derivation: needs to system, system to subsystems ----

	#RequirementDerivation::derivation connection deriveDownlink
		connect specification.needs.science to specification.system.downlink;
	#RequirementDerivation::derivation connection derivePower
		connect specification.needs.science to specification.system.power;
	#RequirementDerivation::derivation connection deriveSurvival
		connect specification.needs.longevity to specification.system.survival;
	#RequirementDerivation::derivation connection deriveArrayOutput
		connect specification.system.power to specification.subsystem.arrayOutput;
	#RequirementDerivation::derivation connection deriveBatteryDepth
		connect specification.system.power to specification.subsystem.batteryDepth;
	#RequirementDerivation::derivation connection deriveLinkMargin
		connect specification.system.downlink to specification.subsystem.linkMargin;
	#RequirementDerivation::derivation connection deriveDataRate
		connect specification.system.downlink to specification.subsystem.dataRate;
	#RequirementDerivation::derivation connection deriveHeaterPower
		connect specification.system.survival to specification.subsystem.heaterPower;
}

package PowerDesign {
	private import ScalarValues::*;
	private import OrbiterVocabulary::*;
	private import ProgramRequirements::*;

	analysis def ArrayDegradationAnalysis;

	part powerSubsystem {
		part array : SolarArray {
			attribute :>> endOfLifePower = 1180.0;
		}
		part battery : Battery {
			attribute :>> depthOfDischarge = 0.38;
		}
		// The team's own derived requirement, kept beside the design it drives.
		requirement <'PWR-3'> cellDegradation : ProgramRequirement {
			doc /* Cell output shall degrade no more than 2.5 percent per year. */
			attribute :>> team = "Power";
			attribute :>> priority = "high";
		}
		satisfy specification.subsystem.arrayOutput by array;
		satisfy specification.subsystem.batteryDepth by battery;
		satisfy cellDegradation by array;
	}
	#RequirementDerivation::derivation connection deriveCellDegradation
		connect specification.subsystem.arrayOutput to powerSubsystem.cellDegradation;
	#ModelingMetadata::refinement dependency degradationRefinement
		from ArrayDegradationAnalysis to powerSubsystem::cellDegradation;

	verification def MeasureArray {
		subject array : SolarArray;
		VerificationCases::PassIf(array.endOfLifePower >= 1100.0)
	}
	verification arrayIlluminationTest : MeasureArray {
		subject array = powerSubsystem.array;
		objective { verify specification.subsystem.arrayOutput; }
	}
	verification def CycleBattery {
		subject battery : Battery;
		VerificationCases::PassIf(battery.depthOfDischarge <= 0.4)
	}
	verification batteryCycleTest : CycleBattery {
		subject battery = powerSubsystem.battery;
		objective { verify specification.subsystem.batteryDepth; }
	}
	verification eclipseCycleAnalysis : CycleBattery {
		subject battery = powerSubsystem.battery;
		objective { verify specification.subsystem.batteryDepth; }
	}
}

package CommsDesign {
	private import ScalarValues::*;
	private import OrbiterVocabulary::*;
	private import ProgramRequirements::*;

	part commsSubsystem {
		part transmitter : Transmitter {
			attribute :>> dataRate = 4.2;
			attribute :>> linkMargin = 2.1;
		}
		satisfy specification.subsystem.linkMargin by transmitter;
		satisfy specification.subsystem.dataRate by transmitter;
		satisfy specification.system.downlink by transmitter;
	}

	verification def LinkBudget {
		subject transmitter : Transmitter;
		VerificationCases::PassIf(transmitter.linkMargin >= 3.0)
	}
	verification linkBudgetAnalysis : LinkBudget {
		subject transmitter = commsSubsystem.transmitter;
		objective { verify specification.subsystem.linkMargin; }
	}
	verification def RateTest {
		subject transmitter : Transmitter;
		VerificationCases::PassIf(transmitter.dataRate >= 4.0)
	}
	verification dataRateTest : RateTest {
		subject transmitter = commsSubsystem.transmitter;
		objective { verify specification.subsystem.dataRate; }
	}
}

package ThermalDesign {
	private import OrbiterVocabulary::*;
	private import ProgramRequirements::*;

	analysis def EclipseThermalAnalysis;

	part thermalSubsystem {
		part heaters : HeaterBank;
	}
	// The heater requirement is refined and analysed, but nothing satisfies or
	// verifies it yet.
	#ModelingMetadata::refinement dependency heaterRefinement
		from HeaterBank to ProgramRequirements::specification::subsystem::heaterPower;
	#ModelingMetadata::refinement dependency eclipseRefinement
		from EclipseThermalAnalysis to ProgramRequirements::specification::system::survival;
}

package ProgramReport {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import ProgramRequirements::*;
	private import PowerDesign::*;
	private import CommsDesign::*;
	private import ThermalDesign::*;

	// ---- The requirement set: the specification tree plus the requirement
	// the power team owns, in one ordered list ----

	// A satisfy usage is a RequirementUsage too, so a root holding both
	// requirements and satisfy statements needs the latter taken out.
	calc def RequirementsUnder :> Query {
		in root : Element;
		Except(
			source = WhereType(source = Descendants(source = root, maxDepth = 2), type = "RequirementUsage"),
			exclude = WhereType(source = Descendants(source = root, maxDepth = 2), type = "SatisfyRequirementUsage")
		)
	}

	calc def AllRequirements :> Query {
		OrderBy(
			source = Union(
				source = RequirementsUnder(root = specification),
				other = RequirementsUnder(root = powerSubsystem)
			),
			property = "shortName",
			direction = "ascending",
			missing = "last",
			multiple = "error"
		)
	}

	// ---- The matrix: identity, ownership, lineage, coverage, one row each ----

	calc def Matrix :> Query {
		Project(
			source = AllRequirements(),
			properties = ("team", "shortName", "name", "priority"),
			columns = (
				RelatedColumn(name = "derivedFrom", relationshipKind = "derivation", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "descendants", relationshipKind = "derivation", direction = "outgoing", maxDepth = 3, aggregate = "count"),
				RelatedColumn(name = "refinedBy", relationshipKind = "refinement", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "verifications", relationshipKind = "verification", direction = "incoming", maxDepth = 1, aggregate = "count"),
				RelatedColumn(name = "satisfied", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1, aggregate = "any")
			)
		)
	}

	// ---- Coverage sets ----

	calc def Identified :> Query {
		in rows : Element[0..*] ordered;
		Project(source = rows, properties = ("team", "shortName", "name", "priority"))
	}

	calc def Unsatisfied :> Query {
		WhereRelated(source = AllRequirements(), relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1, exists = false)
	}

	calc def Unverified :> Query {
		WhereRelated(source = AllRequirements(), relationshipKind = "verification", direction = "incoming", maxDepth = 1, exists = false)
	}

	// Requirements that derive others are met through their children, so only
	// the leaves of the derivation tree count as gaps.
	calc def Parents :> Query {
		WhereRelated(source = AllRequirements(), relationshipKind = "derivation", direction = "outgoing", maxDepth = 1)
	}

	calc def UncoveredLeaves :> Query {
		Except(source = Union(source = Unsatisfied(), other = Unverified()), exclude = Parents())
	}

	calc def UncoveredLeafTable :> Query {
		Identified(rows = UncoveredLeaves())
	}

	calc def CriticalUncovered :> Query {
		Identified(rows = WhereFeature(source = UncoveredLeaves(), 'feature' = "priority", operator = "=", value = "critical"))
	}

	// Satisfied by a part but never verified: claims nothing checks.
	calc def SatisfiedUnverified :> Query {
		Identified(rows = Except(source = Unverified(), exclude = Unsatisfied()))
	}

	// Refined by a design element or analysis while no part satisfies them:
	// work in progress rather than a gap nobody owns.
	calc def RefinedButUnsatisfied :> Query {
		Identified(rows = WhereRelated(source = Unsatisfied(), relationshipKind = "refinement", direction = "incoming", maxDepth = 1))
	}

	// ---- Verdicts across every team's design ----

	calc def DesignVerdicts :> Query {
		Union(
			source = Union(source = Verdicts(source = powerSubsystem), other = Verdicts(source = commsSubsystem)),
			other = Verdicts(source = thermalSubsystem)
		)
	}

	calc def TeamVerdicts :> Query {
		Project(source = DesignVerdicts(), properties = ("kind", "name", "path", "verdict"))
	}

	calc def Failing :> Query {
		Project(
			source = WhereFeature(source = DesignVerdicts(), 'feature' = "verdict", operator = "=", value = "violated"),
			properties = ("kind", "name", "path", "condition")
		)
	}

	// ---- Document ----

	part def ProgramTraceability :> Document {
		attribute redefines title = "Orbiter Program Requirements Traceability";

		part intro : Paragraph {
			attribute redefines text = "Every requirement of the program in one matrix, grouped by the team that owns it, with its lineage, refinements, satisfiers and verification count; then the gaps that remain and the checks that fail.";
		}

		part matrix : Section {
			attribute redefines title = "Traceability matrix";
			part byTeam : Table {
				attribute redefines caption = "Requirements by owning team";
				attribute redefines groupBy = "team";
				calc rows : Matrix;
			}
		}

		part gaps : Section {
			attribute redefines title = "Coverage gaps";
			part leaves : Table {
				attribute redefines caption = "Leaf requirements with no satisfier or no verification";
				calc rows : UncoveredLeafTable;
			}
			part critical : Table {
				attribute redefines caption = "Of those, the critical ones";
				calc rows : CriticalUncovered;
			}
			part unchecked : Table {
				attribute redefines caption = "Satisfied but never verified";
				calc rows : SatisfiedUnverified;
			}
			part inProgress : Table {
				attribute redefines caption = "Unsatisfied but already refined by a design element or analysis";
				calc rows : RefinedButUnsatisfied;
			}
		}

		part verdicts : Section {
			attribute redefines title = "Verdicts";
			part all : Table {
				attribute redefines caption = "Every check across the three designs";
				calc rows : TeamVerdicts;
			}
			part failed : Table {
				attribute redefines caption = "Checks that came out false";
				calc rows : Failing;
			}
		}
	}
}
