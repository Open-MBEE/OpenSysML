package Probe {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import QVTS::*;

	part def Payload {
		attribute mass : Real;
	}
	part probe {
		part camera : Payload {
			attribute :>> mass = 4.5;
		}
		part antenna : Payload {
			attribute :>> mass = 12.0;
		}
		part twin;
		part twin;
	}

	calc def Masses :> Query {
		in root : Element;
		Project(source = WhereType(source = Descendants(source = root), type = "PartUsage"), properties = ("name", "mass"))
	}
	calc def Self :> Query {
		in root : Element;
		Project(source = root, properties = ("name"))
	}
	calc def NoStates :> Query {
		Project(source = WhereType(source = Descendants(source = Named(qualifiedName = "Probe::probe")), type = "StateUsage"), properties = ("name"))
	}
	calc def Misspelled :> Query {
		Project(source = Named(qualifiedName = "Probe::probe::camra"), properties = ("name"))
	}
	calc def Twin :> Query {
		Project(source = Named(qualifiedName = "Probe::probe::twin"), properties = ("name"))
	}

	view heavy : QueryResult {
		expose Probe::probe::antenna;
		attribute :>> query = "Probe::Self root=Probe::probe::antenna";
		attribute :>> source = "probe.sysml";
		attribute :>> outcome = Outcome::established;
		attribute :>> provenance = "urn:prov:bundle:heavy";
	}
	view nothing : QueryResult {
		attribute :>> query = "Probe::NoStates";
		attribute :>> source = "probe.sysml";
		attribute :>> outcome = Outcome::established;
		attribute :>> provenance = "urn:prov:bundle:nothing";
	}
	view plain {
		expose Probe::probe::camera;
	}
}
