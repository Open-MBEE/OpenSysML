package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import Views::*;

	part def Subsystem {
		attribute mass : Real;
		attribute allocatedMass : Real;
		attribute zone : String;
	}

	part def 'Assembly *frame*';

	part telescope : 'Assembly *frame*' {
		part <'M1'> optics : Subsystem {
			doc /* The primary mirror assembly. */
			doc /* Collects light | not *heat*
			     * from the target. */
			attribute redefines mass = 8.5;
			attribute redefines allocatedMass = 10.0;
			attribute redefines zone = "payload";
		}
		part segmentControl : Subsystem {
			doc /* Actuators that phase the mirror segments. */
			attribute redefines mass = 20.0;
			attribute redefines allocatedMass = 25.5;
			attribute redefines zone = "payload";
		}
		part <'M3|*'> mount : Subsystem {
			attribute redefines mass = 15.0;
			attribute redefines allocatedMass = 15.0;
			attribute redefines zone = "support | *frame*";
		}
		part 'baffle|shroud *tricky*' : Subsystem {
			attribute redefines mass = 1.5;
			attribute redefines zone = "support | *frame*";
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

	state def ObservatoryStates {
		entry; then idle;
		state idle;
		state observing;
		transition first idle then observing;
		transition first observing then idle;
	}

	state operatingStates : ObservatoryStates;

	view interconnectView {
		expose imagingChain;
		render asInterconnectionDiagram;
	}

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

	calc def MassMargins :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = Subsystems(root = root),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name"),
			columns = (
				Column(name = "label", expression = "subsystem: " + Element::name),
				Column(
					name = "margin",
					expression = (Subsystem::allocatedMass ?? 0.0) - Subsystem::mass
				)
			)
		)
	}

	calc def StyledHeavyNames :> Query {
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
			properties = ("name"),
			columns = (
				Column(name = "style", expression = "strong"),
				Column(name = "url", expression = "https://example.com/parts#" + Element::name)
			)
		)
	}

	calc def SubsystemNotes :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = Subsystems(root = root),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("shortName", "name", "documentation")
		)
	}

	calc def MissingSubsystems :> Query {
		in root : Element;
		Project(
			source = WhereName(
				source = Subsystems(root = root),
				operator = "=",
				value = "Nonexistent"
			),
			properties = ("name", "mass")
		)
	}

	calc def DeclaredTypes :> Query {
		in root : Element;
		RelatedElements(
			source = root,
			relationshipKind = "typing",
			direction = "outgoing",
			maxDepth = 1
		)
	}

	part def MassReport :> Document {
		attribute redefines title = "Telescope Mass Report";

		part intro : Paragraph {
			attribute redefines text = "Mass rollup for the telescope assembly. Masses are in kg | not #grams, *not* _lbs_, `raw`, <b>&plain</b>.";
		}

		part formatted : Paragraph {
			part lead : Span {
				attribute redefines text = "Reading order:";
			}
			part em : Span {
				attribute redefines text = "em*ph*asis";
				attribute redefines style = "emphasis";
			}
			part bold : Span {
				attribute redefines text = "bold_move";
				attribute redefines style = "strong";
			}
			part code : Span {
				attribute redefines text = "mass >= `limit`";
				attribute redefines style = "code";
			}
			part spec : Link {
				attribute redefines text = "the [spec]";
				attribute redefines target = "https://example.com/spec(v2).md";
			}
			part see : Ref {
				ref redefines target = breakdown;
			}
			part zonesRef : Ref {
				attribute redefines text = "the zone groups";
				ref redefines target = zones;
			}
		}

		part zones : Table {
			attribute redefines caption = "Subsystems grouped by zone";
			attribute redefines groupBy = "zone";
			calc rows : ZonedSubsystems {
				in root = telescope;
			}
		}

		part breakdown : Section {
			attribute redefines title = "Subsystem Masses | by *name*";

			part masses : Table {
				attribute redefines caption = "All subsystems by mass";
				calc rows : SubsystemTable {
					in root = telescope;
				}
			}

			part margins : Table {
				attribute redefines caption = "Mass margins (allocated - estimated)";
				calc rows : MassMargins {
					in root = telescope;
				}
			}

			part notesTable : Table {
				attribute redefines caption = "Subsystem notes";
				calc rows : SubsystemNotes {
					in root = telescope;
				}
			}

			part notes : Definitions {
				attribute redefines term = "shortName";
				attribute redefines description = "documentation";
				calc rows : SubsystemNotes {
					in root = telescope;
				}
			}

			part heavy : Section {
				attribute redefines title = "Heavy Subsystems";

				part summary : Paragraph {
					calc names : HeavySubsystemNames {
						in root = telescope;
						in threshold = "10";
					}
				}

				part heavyItems : List {
					attribute redefines style = "number";
					calc items : HeavySubsystemNames {
						in root = telescope;
						in threshold = "10";
					}
				}

				part styledSummary : Paragraph {
					calc names : StyledHeavyNames {
						in root = telescope;
						in threshold = "10";
					}
					part styledName : SpanColumn {
						attribute redefines column = "name";
						attribute redefines styleColumn = "style";
					}
					part linkedName : LinkColumn {
						attribute redefines column = "name";
						attribute redefines targetColumn = "url";
					}
				}

				part codeItems : List {
					calc items : StyledHeavyNames {
						in root = telescope;
						in threshold = "10";
					}
					part codeName : SpanColumn {
						attribute redefines column = "name";
						attribute redefines style = "code";
					}
				}
			}

			part missing : Section {
				attribute redefines title = "Missing Subsystems";

				part missingTable : Table {
					calc rows : MissingSubsystems {
						in root = telescope;
					}
				}

				part missingItems : List {
					calc items : MissingSubsystems {
						in root = telescope;
					}
				}

				part missingNotes : Definitions {
					attribute redefines term = "name";
					attribute redefines description = "mass";
					calc rows : MissingSubsystems {
						in root = telescope;
					}
				}
			}
		}

		part diagrams : Section {
			attribute redefines title = "Diagrams";

			part imaging : Diagram {
				attribute redefines caption = "Imaging chain interconnection";
				ref redefines source = interconnectView;
			}

			part states : Diagram {
				attribute redefines caption = "Observatory states, left to right";
				attribute redefines kind = "state";
				attribute redefines direction = "LR";
				ref redefines source = operatingStates;
			}
		}

		part typing : Section {
			attribute redefines title = "Declared Types";

			part note : Paragraph {
				attribute redefines text = "The declared type of the telescope, by relationship traversal.";
			}

			part typeTable : Table {
				attribute redefines caption = "Type of telescope";
				calc rows : DeclaredTypes {
					in root = telescope;
				}
			}

			part typeItems : List {
				calc items : DeclaredTypes {
					in root = telescope;
				}
			}
		}
	}
}
