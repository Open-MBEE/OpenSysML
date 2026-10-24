package Icd {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	enum def Status { preliminary; baselined; }
	metadata def Control {
		attribute document : String;
		attribute status : Status;
	}
	part def Link {
		@Control { document = "ICD-001"; status = Status::baselined; }
		attribute voltage : ElectricPotentialValue = 28.0 [V];
		attribute total : ElectricPotentialValue = voltage * 2;
		attribute channels : Natural = 4;
		attribute spare : Natural;
		attribute label : String = "keep-alive";
	}

	calc def Parameters :> Query {
		Project(
			source = WhereType(source = OwnedElements(source = Named(qualifiedName = "Icd::Link")), type = "AttributeUsage"),
			properties = ("name", "valuation")
		)
	}
	calc def Controlled :> Query {
		Project(
			source = Named(qualifiedName = "Icd::Link"),
			properties = ("name", "Control::document"),
			columns = (Column(name = "status", path = "Control::status"))
		)
	}
}
