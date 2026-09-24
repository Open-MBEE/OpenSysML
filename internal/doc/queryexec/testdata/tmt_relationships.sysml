package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Subsystem {
		attribute mass : Real;
	}
	part def OpticalSubsystem :> Subsystem;
	part def MirrorAssembly :> OpticalSubsystem {
		attribute :>> mass = 10.0;
	}

	port def OpticalPort;
	port def DataPort;

	part telescope {
		part primaryMirror : MirrorAssembly {
			port opticalOut : OpticalPort;
		}
		part instrumentCluster : Subsystem {
			port opticalIn : OpticalPort;
			port dataOut : DataPort;
		}
		part mountControl : Subsystem {
			port dataIn : DataPort;
		}
		connection opticalPath connect primaryMirror.opticalOut to instrumentCluster.opticalIn;
		connect instrumentCluster.dataOut to mountControl.dataIn;
	}

	part instruments;
	part iris :> instruments;
	part modhis :> instruments;

	part def Computer;
	part scienceComputer : Computer;
	allocation processing allocate telescope.instrumentCluster to scienceComputer;

	requirement def DataRequirement;
	requirement massRequirement;
	requirement pointingRequirement;

	part observatory {
		satisfy massRequirement by telescope;
	}
	part groundStation {
		satisfy pointingRequirement;
	}

	verification def MassTest;
	verification massVerification : MassTest {
		objective {
			verify massRequirement;
		}
	}

	part mountControlStation : Computer;
	part dataArchive {
		satisfy requirement archiveRequirement : DataRequirement by scienceComputer;
	}
	part fieldStation {
		satisfy requirement by mountControlStation;
	}
	part relayHub;
	part relayControl {
		satisfy requirement relayRequirement :> massRequirement by relayHub;
	}
	verification archiveVerification : MassTest {
		objective archiveObjective {
			verify requirement archiveCheck by scienceComputer;
		}
	}
	verification fieldVerification : MassTest {
		objective {
			verify requirement by mountControlStation;
		}
	}

	calc def Related :> Query {
		in source : Element[1..*] ordered;
		in kind : String;
		in direction : String;
		in maxDepth : Integer;
		RelatedElements(
			source = source,
			relationshipKind = kind,
			direction = direction,
			maxDepth = maxDepth
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

	calc def SpecializerReport :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = RelatedElements(
					source = root,
					relationshipKind = "specialization",
					direction = "incoming",
					maxDepth = 2
				),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name", "qualifiedName")
		)
	}

	calc def InvokedSatisfiers :> Query {
		in req : Element;
		WhereType(source = SatisfiedBy(req = req), type = "PartUsage")
	}
}
