package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Component {
		attribute v : Integer default 1;
		assert constraint ok { v < 10 }
	}
	part def Assembly {
		part subsystem : Component[*];
		part small : Component :> subsystem {
			attribute :>> v = 99;
		}
		part smalls : Component[2] :> subsystem {
			attribute :>> v = 50;
		}
	}
	part assembly : Assembly;

	calc def Checks :> Query {
		in root : Element;
		Project(source = Verdicts(source = root), properties = ("path", "verdict"))
	}
}
