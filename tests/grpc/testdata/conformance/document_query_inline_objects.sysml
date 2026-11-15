package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Wheel {
		attribute pressure : Integer default 30;
		assert constraint inflated { pressure >= 25 }
	}
	part def Car {
		attribute mass : Integer = 1200;
		part wheels : Wheel[2];
		assert constraint light { mass < 1500 }
	}
	part car : Car;
	part spare : Wheel {
		attribute :>> pressure = 20;
	}

	calc def Parts :> Query {
		in root : Element;
		Project(source = OwnedElements(source = root), properties = ("name", "pressure"))
	}
	calc def Drive :> Query {
		in root : Element;
		Project(source = root, properties = ("mass", "wheels"))
	}
	calc def Pressure :> Query {
		in root : Element;
		Project(source = root, properties = ("name", "pressure"))
	}
	calc def Wheels :> Query {
		OrderBy(
			source = Project(source = Objects(type = "Wheel"), properties = ("pressure")),
			property = "pressure",
			direction = "ascending",
			missing = "last",
			multiple = "first"
		)
	}
	calc def Checks :> Query {
		in root : Element;
		Project(source = Verdicts(source = root), properties = ("path", "verdict"))
	}
}
