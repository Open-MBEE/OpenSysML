package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Subsystem {
		attribute mass : Real;
	}

	part telescope {
		part optics : Subsystem {
			attribute redefines mass = 8.5;
		}
		part segmentControl : Subsystem {
			attribute redefines mass = 20.0;
		}
		part mount : Subsystem {
			attribute redefines mass = 15.0;
		}
	}

	part instruments {
		part spectrograph : Subsystem {
			attribute redefines mass = 4.0;
		}
	}

	calc def Subsystems :> Query {
		in root : Element = telescope;
		WhereType(
			source = Descendants(source = root, maxDepth = 3),
			type = "PartUsage"
		)
	}

	calc def HeavySubsystems :> Query {
		in root : Element = telescope;
		in threshold : String default "10";
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
			properties = ("name", "mass")
		)
	}

	calc def LightSubsystems :> HeavySubsystems {
		in redefines threshold default "5";
	}

	part def DefaultedReport :> Document {
		attribute redefines title = "Defaulted Queries";

		part heavy : Table {
			attribute redefines caption = "Heavy subsystems by default";
			calc rows : HeavySubsystems;
		}

		part light : List {
			calc items : LightSubsystems;
		}

		part instrumentsList : List {
			calc items : HeavySubsystems {
				in root = instruments;
				in threshold = "1";
			}
		}
	}
}
