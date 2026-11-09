package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Wheel {
		attribute pressure : Integer default 30;
	}
	part def Engine {
		attribute power : Integer default 100;
	}
	part def Car {
		part engine : Engine;
		part wheels : Wheel[2];
	}
	part car : Car;
	part spare : Wheel {
		attribute :>> pressure = 20;
	}

	calc def Parts :> Query {
		in root : Element;
		Project(
			source = Descendants(source = root, maxDepth = 2),
			properties = ("name", "qualifiedName", "pressure")
		)
	}
	calc def Drive :> Query {
		in root : Element;
		Project(source = root, properties = ("name", "engine", "wheels"))
	}
	calc def Wheels :> Query {
		OrderBy(
			source = Project(source = Objects(type = "Wheel"), properties = ("qualifiedName", "pressure")),
			property = "pressure",
			direction = "ascending",
			missing = "last",
			multiple = "first"
		)
	}

	part def CarReport :> Document {
		attribute redefines title = "Car Report";

		part intro : Paragraph {
			attribute redefines text = "The objects a session holds, by path.";
		}
		part parts : Table {
			attribute redefines caption = "Parts of the car";
			calc rows : Parts {
				in root = car;
			}
		}
		part drive : Table {
			attribute redefines caption = "The car's engine and wheels";
			calc rows : Drive {
				in root = car;
			}
		}
		part wheels : List {
			calc items : Wheels;
		}
	}
}
