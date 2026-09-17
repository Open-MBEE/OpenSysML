package Cookbook {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import SI::*;

	// ---- The model the recipes query ----

	part def Subsystem {
		attribute mass : Real;
		assert constraint massKnown { mass > 0.0 }
	}
	part def OpticalSubsystem :> Subsystem;
	part def MirrorAssembly :> OpticalSubsystem {
		attribute :>> mass = 10.0;
		assert constraint lightweight { mass < 5.0 }
	}

	metadata def Critical;

	port def OpticalPort;
	port def DataPort;

	part telescope {
		part primaryMirror : MirrorAssembly {
			@Critical;
			port opticalOut : OpticalPort;
		}
		part instrumentCluster : Subsystem {
			attribute redefines mass = 4.5;
			port opticalIn : OpticalPort;
			port dataOut : DataPort;
		}
		part mountControl : Subsystem {
			attribute redefines mass = 15.0;
			port dataIn : DataPort;
		}
		connection opticalPath connect primaryMirror.opticalOut to instrumentCluster.opticalIn;
		connection dataPath connect instrumentCluster.dataOut to mountControl.dataIn;
	}

	part def Computer;
	part scienceComputer : Computer;
	allocation processing allocate telescope.instrumentCluster to scienceComputer;

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

	// ---- The run the state and event recipes query ----

	attribute def Open;
	attribute def Close;
	attribute def Slew { attribute azimuth : Real; }

	state def DomeControl {
		attribute azimuth : Real = 0.0;
		entry; then closed;
		state closed;
		transition closed_open first closed accept Open then open;
		state open parallel {
			state pointing {
				entry; then parked;
				state parked;
				transition parked_slewing first parked accept s : Slew do assign azimuth := s.azimuth then slewing;
				state slewing;
			}
			state shutter {
				entry; then opening;
				state opening;
				transition opening_opened first opening accept after 1 [s] then opened;
				state opened;
			}
		}
		transition open_close first open accept Close then closed;
	}
	part def Dome { exhibit state control : DomeControl; }
	part dome : Dome;
	part spareDome : Dome;

	// ---- Recipe queries ----

	calc def Children :> Query {
		in root : Element;
		OwnedElements(source = root)
	}

	calc def AllParts :> Query {
		in root : Element;
		WhereType(
			source = Descendants(source = root, maxDepth = 10),
			type = "PartUsage"
		)
	}

	calc def Enclosing :> Query {
		in leaf : Element;
		Ancestors(source = leaf, maxDepth = 2)
	}

	calc def Connections :> Query {
		in root : Element;
		WhereType(
			source = Descendants(source = root, maxDepth = 10),
			type = "ConnectionUsage"
		)
	}

	calc def MirrorParts :> Query {
		in root : Element;
		WhereName(
			source = AllParts(root = root),
			operator = "contains",
			value = "Mirror"
		)
	}

	calc def CriticalParts :> Query {
		in root : Element;
		WhereMetadata(
			source = AllParts(root = root),
			'metadata' = "Cookbook::Critical"
		)
	}

	calc def HeavyParts :> Query {
		in root : Element;
		in threshold : String;
		WhereFeature(
			source = AllParts(root = root),
			'feature' = "mass",
			operator = ">=",
			value = threshold
		)
	}

	calc def PartsByMass :> Query {
		in root : Element;
		OrderBy(
			source = AllParts(root = root),
			property = "mass",
			direction = "descending",
			missing = "last",
			multiple = "error"
		)
	}

	calc def MassTable :> Query {
		in root : Element;
		Project(
			source = PartsByMass(root = root),
			properties = ("name", "mass", "qualifiedName")
		)
	}

	calc def MassBudget :> Query {
		in root : Element;
		Project(
			source = PartsByMass(root = root),
			properties = ("name", "mass"),
			columns = (
				Column(name = "massLbs", expression = (Subsystem::mass ?? 0.0) * 2.2),
				Column(name = "label", expression = "part: " + Element::name)
			)
		)
	}

	calc def ConnectedTo :> Query {
		in origin : Element;
		RelatedElements(
			source = origin,
			relationshipKind = "connection",
			direction = "outgoing",
			maxDepth = 1
		)
	}

	calc def AllocatedTargets :> Query {
		in origin : Element;
		RelatedElements(
			source = origin,
			relationshipKind = "allocation",
			direction = "outgoing",
			maxDepth = 1
		)
	}

	calc def SatisfiedBy :> Query {
		in req : Element;
		RelatedElements(
			source = req,
			relationshipKind = "satisfaction",
			direction = "incoming",
			maxDepth = 1
		)
	}

	calc def VerifiedBy :> Query {
		in req : Element;
		RelatedElements(
			source = req,
			relationshipKind = "verification",
			direction = "incoming",
			maxDepth = 1
		)
	}

	calc def Specializers :> Query {
		in general : Element;
		RelatedElements(
			source = general,
			relationshipKind = "specialization",
			direction = "incoming",
			maxDepth = 2
		)
	}

	calc def HeldParts :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 2), type = "Subsystem"),
			properties = ("qualifiedName", "mass")
		)
	}

	calc def HeldSubsystems :> Query {
		OrderBy(
			source = Project(source = Objects(type = "Subsystem"), properties = ("qualifiedName", "mass")),
			property = "mass",
			direction = "descending",
			missing = "last",
			multiple = "first"
		)
	}

	calc def Checks :> Query {
		in root : Element;
		Project(source = Verdicts(source = root), properties = ("path", "name", "verdict", "reason"))
	}

	calc def Violated :> Query {
		in root : Element;
		WhereFeature(source = Verdicts(source = root), 'feature' = "verdict", operator = "=", value = "violated")
	}

	// ---- A requirement nothing satisfies, and the coverage recipes ----

	requirement def PointingRequirement {
		doc /* The telescope holds a target within the stated accuracy. */
	}
	requirement <'REQ-2'> pointingRequirement : PointingRequirement {
		doc /* The telescope points at a target to within 2 arcseconds. */
		requirement <'REQ-2.1'> slewRequirement {
			doc /* The mount reaches a new target within 60 seconds. */
		}
		requirement <'REQ-2.2'> trackingRequirement {
			doc /* The mount tracks a target for 30 minutes without drift. */
		}
	}
	verification def PointingTest;
	verification pointingVerification : PointingTest {
		objective {
			verify pointingRequirement;
		}
	}

	calc def Requirements :> Query {
		in root : Element;
		Except(
			source = Union(
				source = WhereType(source = Descendants(source = root, maxDepth = 10), type = "RequirementDefinition"),
				other = WhereType(source = Descendants(source = root, maxDepth = 10), type = "RequirementUsage")
			),
			exclude = WhereType(source = Descendants(source = root, maxDepth = 10), type = "SatisfyRequirementUsage")
		)
	}

	calc def UnsatisfiedRequirements :> Query {
		in root : Element;
		WhereRelated(
			source = Requirements(root = root),
			relationshipKind = "satisfaction",
			direction = "incoming",
			maxDepth = 1,
			exists = false
		)
	}

	calc def UnverifiedRequirements :> Query {
		in root : Element;
		WhereRelated(
			source = Requirements(root = root),
			relationshipKind = "verification",
			direction = "incoming",
			maxDepth = 1,
			exists = false
		)
	}

	calc def UncoveredRequirements :> Query {
		in root : Element;
		Union(
			source = UnsatisfiedRequirements(root = root),
			other = UnverifiedRequirements(root = root)
		)
	}

	calc def VerifiedButUnsatisfied :> Query {
		in root : Element;
		Except(
			source = UnsatisfiedRequirements(root = root),
			exclude = UnverifiedRequirements(root = root)
		)
	}

	calc def RequirementTree :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = Requirements(root = root),
				property = "qualifiedName",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("shortName", "name", "documentation")
		)
	}

	calc def DomeStates :> Query {
		in root : Element;
		Project(source = States(source = root), properties = ("machine", "statePath", "region", "enclosing"))
	}

	calc def Opened :> Query {
		Project(source = InState(name = "open"), properties = ("qualifiedName"))
	}

	calc def Accepted :> Query {
		in root : Element;
		Project(
			source = Events(source = root, kind = "accept", since = 1 [s], before = 2.5 [s]),
			properties = ("time", "event", "payload")
		)
	}
}
