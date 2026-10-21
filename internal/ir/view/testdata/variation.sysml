// A variation point is drawn with the «variation» keyword before its kind
// (SysML v2.1 §7.6.7, Table 4), its variants with «variant»; the variation
// point is abstract without the keyword being written.
package VariationViews {
	private import Views::*;
	private import OpenSysMLRenderings::*;

	part def Engine;
	part def Transmission;
	part fourCylinder : Engine;
	part sixCylinder : Engine;
	part manual : Transmission;
	part automatic : Transmission;

	variation part def EngineChoices :> Engine {
		variant fourCylinder;
		variant sixCylinder;
	}

	part vehicleFamily {
		part engine : EngineChoices;
		variation part transmission : Transmission {
			variant manual;
			variant automatic;
		}
		connect engine to transmission;
	}

	view def Family;
	view familyTree : Family {
		expose vehicleFamily::**;
		expose EngineChoices::**;
		render asTreeDiagram;
	}
	view familyLinks : Family {
		expose vehicleFamily::**;
		render asInterconnectionDiagram;
	}
	view familyTable : Family {
		expose vehicleFamily::**;
		expose EngineChoices::**;
		render asElementTable;
	}
	view familyMixed : Family {
		expose vehicleFamily::**;
		expose EngineChoices::**;
		render asMixedDiagram;
	}
}
