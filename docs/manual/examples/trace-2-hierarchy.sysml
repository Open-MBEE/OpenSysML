// Traceability over a requirement hierarchy: a lander specification whose
// requirements nest two levels deep, parts nested inside parts that satisfy
// the leaves, verification cases that pass or fail against the design's
// values, and a report with the tree, a matrix over the whole tree, the
// coverage gaps and the verdicts that came out false.
package LanderHierarchy {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	// ---- Design vocabulary ----

	part def Lander {
		attribute dryMass : Real;
	}
	part def DescentEngine {
		attribute thrust : Real;
	}
	part def LandingLeg {
		attribute strokeLength : Real;
	}
	part def Radar {
		attribute maxRange : Real;
	}
	part def Transponder;

	// ---- Requirement definitions with checkable conditions ----

	requirement def MassLimit {
		subject lander : Lander;
		require constraint { lander.dryMass <= 1800.0 }
	}
	requirement def ThrustMargin {
		subject engine : DescentEngine;
		require constraint { engine.thrust >= 3000.0 }
	}
	requirement def StrokeLength {
		subject leg : LandingLeg;
		require constraint { leg.strokeLength >= 0.3 }
	}
	requirement def RadarRange {
		subject radar : Radar;
		require constraint { radar.maxRange >= 5000.0 }
	}

	// ---- The specification: a tree of requirements ----

	part specification {
		requirement <'L-1'> mass : MassLimit {
			doc /* The lander dry mass shall not exceed 1800 kg. */
		}
		requirement <'L-2'> descent {
			doc /* The lander shall descend under powered control to touchdown. */
			requirement <'L-2.1'> thrust : ThrustMargin {
				doc /* The descent engine shall deliver at least 3000 N. */
			}
			requirement <'L-2.2'> altimetry : RadarRange {
				doc /* The landing radar shall range the surface from 5000 m. */
			}
		}
		requirement <'L-3'> touchdown {
			doc /* The lander shall survive touchdown at 2 m/s. */
			requirement <'L-3.1'> legStroke : StrokeLength {
				doc /* Each landing leg shall absorb 0.3 m of stroke. */
			}
			requirement <'L-3.2'> tipOver {
				doc /* The lander shall not tip over on a 15 degree slope. */
			}
		}
		requirement <'L-4'> beacon {
			doc /* The lander shall transmit a recovery beacon after landing. */
		}
	}

	// ---- The design: parts nested inside parts ----

	part lander : Lander {
		attribute :>> dryMass = 1750.0;

		part propulsion {
			part engine : DescentEngine {
				attribute :>> thrust = 2800.0;
			}
			satisfy specification.descent.thrust by engine;
		}

		part gear {
			part leg : LandingLeg {
				attribute :>> strokeLength = 0.35;
			}
			satisfy specification.touchdown.legStroke by leg;
		}

		part avionics {
			part radar : Radar {
				attribute :>> maxRange = 6000.0;
			}
			part transponder : Transponder;
			satisfy specification.descent.altimetry by radar;
			satisfy specification.beacon by transponder;
		}
	}
	satisfy specification.mass by lander;

	// ---- Verification cases: two pass, one fails, one requirement has none ----

	verification def WeighLander {
		subject vehicle : Lander;
		VerificationCases::PassIf(vehicle.dryMass <= 1800.0)
	}
	verification weighLander : WeighLander {
		subject vehicle = lander;
		objective {
			verify specification.mass;
		}
	}

	verification def FireEngine {
		subject engine : DescentEngine;
		VerificationCases::PassIf(engine.thrust >= 3000.0)
	}
	verification hotFire : FireEngine {
		subject engine = lander.propulsion.engine;
		objective {
			verify specification.descent.thrust;
		}
	}

	verification def DropLeg {
		subject leg : LandingLeg;
		VerificationCases::PassIf(leg.strokeLength >= 0.3)
	}
	verification dropTest : DropLeg {
		subject leg = lander.gear.leg;
		objective {
			verify specification.touchdown.legStroke;
		}
	}

	verification def RangeRadar {
		subject radar : Radar;
		VerificationCases::PassIf(radar.maxRange >= 5000.0)
	}
	verification radarRangeTest : RangeRadar {
		subject radar = lander.avionics.radar;
		objective {
			verify specification.descent.altimetry;
		}
	}

	// ---- Queries ----

	// Every requirement under the root, at any depth. Descendants visits level
	// by level; sorting by the ids, which extend their parent's, restores the tree.
	calc def Requirements :> Query {
		in root : Element;
		OrderBy(
			source = WhereType(source = Descendants(source = root, maxDepth = 3), type = "RequirementUsage"),
			property = "shortName",
			direction = "ascending",
			missing = "last",
			multiple = "error"
		)
	}

	calc def Tree :> Query {
		in root : Element;
		Project(source = Requirements(root = root), properties = ("shortName", "name", "documentation"))
	}

	calc def Matrix :> Query {
		in root : Element;
		Project(
			source = Requirements(root = root),
			properties = ("shortName", "name"),
			columns = (
				RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1),
				RelatedColumn(name = "verifiedBy", relationshipKind = "verification", direction = "incoming", maxDepth = 1)
			)
		)
	}

	calc def Unsatisfied :> Query {
		in root : Element;
		Project(
			source = WhereRelated(
				source = Requirements(root = root),
				relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1, exists = false
			),
			properties = ("shortName", "name")
		)
	}

	calc def Unverified :> Query {
		in root : Element;
		Project(
			source = WhereRelated(
				source = Requirements(root = root),
				relationshipKind = "verification", direction = "incoming", maxDepth = 1, exists = false
			),
			properties = ("shortName", "name")
		)
	}

	// Requirements with a gap of either kind, each listed once.
	calc def AnyGap :> Query {
		in root : Element;
		Union(source = Unsatisfied(root = root), other = Unverified(root = root))
	}

	// Requirements a part satisfies that no case verifies: the design claims
	// them, but nothing checks the claim.
	calc def SatisfiedButUnverified :> Query {
		in root : Element;
		Except(source = Unverified(root = root), exclude = Unsatisfied(root = root))
	}

	calc def Failures :> Query {
		in root : Element;
		Project(
			source = WhereFeature(source = Verdicts(source = root), 'feature' = "verdict", operator = "=", value = "violated"),
			properties = ("kind", "name", "path", "condition")
		)
	}

	calc def AllVerdicts :> Query {
		in root : Element;
		Project(source = Verdicts(source = root), properties = ("kind", "name", "path", "verdict"))
	}

	// ---- Document ----

	part def HierarchyReport :> Document {
		attribute redefines title = "Lander Requirement Traceability";

		part intro : Paragraph {
			attribute redefines text = "The lander specification as a tree, what satisfies and verifies each requirement, where coverage is missing, and which checks fail.";
		}

		part tree : Section {
			attribute redefines title = "Requirement hierarchy";
			part text : Definitions {
				attribute redefines term = "shortName";
				attribute redefines description = "documentation";
				calc rows : Tree {
					in root = specification;
				}
			}
		}

		part matrix : Section {
			attribute redefines title = "Traceability matrix";
			part table : Table {
				attribute redefines caption = "Every requirement with its satisfiers and verifiers";
				calc rows : Matrix {
					in root = specification;
				}
			}
		}

		part coverage : Section {
			attribute redefines title = "Coverage gaps";
			part unsatisfiedTable : Table {
				attribute redefines caption = "Requirements no part satisfies";
				calc rows : Unsatisfied {
					in root = specification;
				}
			}
			part unverifiedTable : Table {
				attribute redefines caption = "Requirements no case verifies";
				calc rows : Unverified {
					in root = specification;
				}
			}
			part uncoveredTable : Table {
				attribute redefines caption = "Requirements with a gap of either kind";
				calc rows : AnyGap {
					in root = specification;
				}
			}
			part uncheckedTable : Table {
				attribute redefines caption = "Requirements satisfied but never verified";
				calc rows : SatisfiedButUnverified {
					in root = specification;
				}
			}
		}

		part verdicts : Section {
			attribute redefines title = "Verdicts";
			part all : Table {
				attribute redefines caption = "Every check of the lander against its requirements";
				calc rows : AllVerdicts {
					in root = lander;
				}
			}
			part failed : Table {
				attribute redefines caption = "Checks that came out false";
				calc rows : Failures {
					in root = lander;
				}
			}
		}
	}
}
