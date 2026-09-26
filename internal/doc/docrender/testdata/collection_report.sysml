package Calibration {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Scenario {
		attribute durations : Real[0..*];
		attribute label : String;
	}
	part campaign {
		part nominal : Scenario {
			attribute :>> durations = (69.0, 98.0);
			attribute :>> label = "nominal";
		}
		part idle : Scenario {
			attribute :>> label = "idle";
		}
		part single : Scenario {
			attribute :>> durations = (36.0);
			attribute :>> label = "single";
		}
	}

	calc def Timings :> Query {
		in root : Element = campaign;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name"),
			columns = (
				Column(name = "durations", expression = Scenario::durations),
				Column(name = "label", expression = Scenario::label)
			)
		)
	}

	part def TimingReport :> Document {
		attribute redefines title = "Calibration Timings";

		part timings : Table {
			attribute redefines caption = "Durations per scenario";
			calc rows : Timings;
		}
	}
}
