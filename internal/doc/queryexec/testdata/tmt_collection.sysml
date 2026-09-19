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

	calc def HeavySubsystems :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = WhereFeature(
					source = WhereType(
						source = Descendants(source = root, maxDepth = 3),
						type = "PartUsage"
					),
					'feature' = "mass",
					operator = ">=",
					value = "10"
				),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name", "qualifiedName", "mass")
		)
	}
}
