package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import Views::*;

	// ---- The system model the report is generated from ----

	part def Subsystem {
		attribute mass : Real;
		attribute zone : String;
	}

	part telescope {
		part optics : Subsystem {
			attribute redefines mass = 8.5;
			attribute redefines zone = "payload";
		}
		part segmentControl : Subsystem {
			attribute redefines mass = 20.0;
			attribute redefines zone = "payload";
		}
		part mount : Subsystem {
			attribute redefines mass = 15.0;
			attribute redefines zone = "support";
		}
	}

	port def DataPort;

	part def Camera {
		port output : DataPort;
	}

	part def Recorder {
		port input : DataPort;
	}

	part imagingChain {
		part camera : Camera;
		part recorder : Recorder;
		connection link connect camera.output to recorder.input;
	}

	requirement massRequirement;
	part observatory {
		satisfy massRequirement by telescope;
	}

	verification def MassTest;
	verification massVerification : MassTest {
		objective {
			verify massRequirement;
		}
	}

	view interconnectView {
		expose imagingChain;
		render asInterconnectionDiagram;
	}

	// ---- Document queries ----

	calc def Subsystems :> Query {
		in root : Element;
		WhereType(
			source = Descendants(source = root, maxDepth = 3),
			type = "PartUsage"
		)
	}

	calc def SubsystemTable :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = Subsystems(root = root),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name", "mass")
		)
	}

	calc def HeavySubsystemNames :> Query {
		in root : Element;
		in threshold : String;
		Project(
			source = OrderBy(
				source = WhereFeature(
					source = Subsystems(root = root),
					'feature' = "mass",
					operator = ">=",
					value = threshold
				),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name")
		)
	}

	calc def ZonedSubsystems :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = Subsystems(root = root),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("zone", "name", "mass")
		)
	}

	calc def SatisfyingParts :> Query {
		in req : Element;
		Project(
			source = RelatedElements(
				source = req,
				relationshipKind = "satisfaction",
				direction = "incoming",
				maxDepth = 1
			),
			properties = ("name", "qualifiedName")
		)
	}

	calc def VerifyingElements :> Query {
		in req : Element;
		Project(
			source = RelatedElements(
				source = req,
				relationshipKind = "verification",
				direction = "incoming",
				maxDepth = 1
			),
			properties = ("qualifiedName")
		)
	}

	// ---- The document definition ----

	part def MassReport :> Document {
		attribute redefines title = "Telescope Mass Report";

		part intro : Paragraph {
			:>> text = "Mass rollup and requirement status for the telescope assembly.";
		}

		part guide : Paragraph {
			part lead : Span {
				:>> text = "This report is";
			}
			part generated : Span {
				:>> text = "generated";
				attribute redefines style = "emphasis";
			}
			part sourceNote : Span {
				:>> text = "from the model by";
			}
			part tool : Span {
				:>> text = "sysml -render-document";
				attribute redefines style = "code";
			}
			part docsLink : Link {
				:>> text = "(OpenSysML)";
				attribute redefines target = "https://opensysml.org/";
			}
			part see : Span {
				:>> text = "— masses are tabulated in";
			}
			part massesRef : Ref {
				ref redefines target = breakdown;
			}
		}

		part breakdown : Section {
			attribute redefines title = "Subsystem Masses";

			part masses : Table {
				attribute redefines caption = "All subsystems by mass";
				calc rows : SubsystemTable {
					in root = telescope;
				}
			}

			part zones : Table {
				attribute redefines caption = "Subsystems grouped by zone";
				attribute redefines groupBy = "zone";
				calc rows : ZonedSubsystems {
					in root = telescope;
				}
			}

			part heavy : Section {
				attribute redefines title = "Heavy Subsystems";

				part note : Paragraph {
					:>> text = "Subsystems at or above 10 kg:";
				}

				part heavyItems : List {
					attribute redefines style = "number";
					calc items : HeavySubsystemNames {
						in root = telescope;
						in threshold = "10";
					}
				}
			}
		}

		part requirements : Section {
			attribute redefines title = "Mass Requirement";

			part satisfiers : Table {
				attribute redefines caption = "Parts satisfying the mass requirement";
				calc rows : SatisfyingParts {
					in req = massRequirement;
				}
			}

			part verifiers : Table {
				attribute redefines caption = "Verifications of the mass requirement";
				calc rows : VerifyingElements {
					in req = massRequirement;
				}
			}
		}

		part diagrams : Section {
			attribute redefines title = "Diagrams";

			part imaging : Diagram {
				attribute redefines caption = "Imaging chain interconnection";
				ref redefines source = interconnectView;
			}

			part structure : Diagram {
				attribute redefines caption = "Telescope part tree, left to right";
				attribute redefines kind = "tree";
				attribute redefines direction = "LR";
				ref redefines source = telescope;
			}
		}
	}
}
