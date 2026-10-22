// An abstract definition or usage is drawn with the «abstract» keyword before
// its kind (SysML v2.1 §7.6.2, §7.6.3, Table 4); a variation is drawn with
// «variation» alone, abstract without that keyword being written (§7.6.7).
package AbstractViews {
	private import Views::*;
	private import OpenSysMLRenderings::*;

	abstract part def Vehicle {
		attribute mass : ScalarValues::Real;
	}
	part def Car :> Vehicle;
	part def Engine;
	abstract port def FuelPort;

	abstract part vehicleFamily : Vehicle {
		abstract port fuel : FuelPort;
		abstract part engine : Engine;
		part chassis;
		connect engine to chassis;
	}
	part sedan : Car :> vehicleFamily;
	variation part def Models :> Vehicle {
		variant sedan;
	}

	view def Family;
	view familyTree : Family {
		expose Vehicle::**;
		expose vehicleFamily::**;
		expose Models::**;
		render asTreeDiagram;
	}
	view familyLinks : Family {
		expose vehicleFamily::**;
		render asInterconnectionDiagram;
	}
	view familyTable : Family {
		expose Vehicle::**;
		expose vehicleFamily::**;
		expose Models::**;
		render asElementTable;
	}
	view familyMixed : Family {
		expose Vehicle::**;
		expose vehicleFamily::**;
		expose Models::**;
		render asMixedDiagram;
	}
}
