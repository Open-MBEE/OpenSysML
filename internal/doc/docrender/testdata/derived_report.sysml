package Derived {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;

	part def Stage {
		attribute dryMass :> ISQ::mass;
		attribute propellantMass :> ISQ::mass;
		attribute mass :> ISQ::mass = dryMass + propellantMass;
		attribute engines : Integer default 1;
		attribute class : String = if mass > 1000000 [kg] ? "heavy" else "light";
	}
	part def FirstStage :> Stage {
		attribute :>> dryMass default = 130000 [kg];
		attribute :>> propellantMass = 2160000 [kg];
		attribute :>> engines default = 5;
	}
	part def UpperStage :> Stage {
		attribute :>> dryMass = 15000 [kg];
		attribute :>> propellantMass = 104000 [kg];
	}
	part def Vehicle {
		part s1 : FirstStage;
		part s2 : FirstStage {
			attribute :>> dryMass = 120000 [kg];
			attribute :>> engines = 3;
		}
		part s3 : UpperStage;
		attribute liftoffMass :> ISQ::mass = s1.mass + s2.mass + s3.mass;
		attribute engineCount : Integer = s1.engines + s2.engines + s3.engines;
	}
	part rocket {
		part s1 : FirstStage;
		part s2 : FirstStage {
			attribute :>> dryMass = 120000 [kg];
			attribute :>> engines = 3;
		}
		part s3 : UpperStage;
	}
	part fleet {
		part rocket : Vehicle;
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
			properties = ("name", "dryMass", "mass", "engines", "class"),
			columns = (Column(name = "perEngine", expression = Stage::mass / Stage::engines))
		)
	}

	calc def Heavy :> Query {
		in root : Element = rocket;
		Project(
			source = WhereFeature(
				source = Stages(root = root),
				'feature' = "mass",
				operator = ">",
				value = "2285000"
			),
			properties = ("name", "mass")
		)
	}

	calc def Vehicles :> Query {
		in root : Element = fleet;
		Project(
			source = Stages(root = root),
			properties = ("name", "liftoffMass", "engineCount")
		)
	}

	part def MassReport :> Document {
		attribute redefines title = "Derived Stage Masses";

		part all : Table {
			attribute redefines caption = "Every stage, heaviest first";
			calc rows : Masses;
		}

		part heavy : List {
			calc items : Heavy;
		}

		part liftoff : Definitions {
			attribute redefines term = "name";
			attribute redefines description = "liftoffMass";
			calc rows : Vehicles;
		}
	}
}
