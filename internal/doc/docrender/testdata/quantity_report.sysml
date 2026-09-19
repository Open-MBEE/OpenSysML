package Launcher {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;

	part def Stage {
		attribute mass :> ISQ::mass;
	}
	part def FirstStage :> Stage {
		attribute :>> mass = 2290000 [kg];
	}
	part def UpperStage :> Stage {
		attribute :>> mass = 119000 [kg];
	}
	part def Probe :> Stage {
		attribute :>> mass = 500000 [g];
	}
	part rocket {
		part s1 : FirstStage;
		part s2 : UpperStage;
		part probe : Probe;
	}

	calc def Stages :> Query {
		in root : Element = rocket;
		WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage")
	}

	calc def Masses :> Query {
		in root : Element = rocket;
		Project(
			source = OrderBy(
				source = Stages(root = root),
				property = "mass",
				direction = "descending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name", "mass"),
			columns = (Column(name = "tonnes", expression = Stage::mass / 1000))
		)
	}

	calc def Heavy :> Query {
		in root : Element = rocket;
		Project(
			source = WhereFeature(
				source = Stages(root = root),
				'feature' = "mass",
				operator = ">=",
				value = "1000000"
			),
			properties = ("name", "mass")
		)
	}

	calc def Margin :> Query {
		in root : Element = rocket;
		in budget : MassValue;
		Project(
			source = Stages(root = root),
			properties = ("name"),
			columns = (Column(name = "margin", expression = budget - Stage::mass))
		)
	}

	calc def Named :> Query {
		in root : Element = rocket;
		in label : String;
		WhereFeature(source = Stages(root = root), 'feature' = "name", operator = "=", value = label)
	}

	part def MassReport :> Document {
		attribute redefines title = "Stage Masses";

		part all : Table {
			attribute redefines caption = "Every stage, heaviest first";
			calc rows : Masses;
		}

		part heavy : Table {
			attribute redefines caption = "Stages of at least a million kilograms";
			calc rows : Heavy;
		}
	}
}
