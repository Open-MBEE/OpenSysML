package Fleet {
	part def Wheel;

	part def Engine {
		attribute power;
		part cylinder[4] : Cylinder;
	}

	part def Cylinder;

	part def Truck {
		part engine : Engine;
		part wheels[6] : Wheel;
	}
}

package TableViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	// States the tabular rendering by name.
	view partsTable {
		expose Fleet::Truck;
		render asElementTable;
	}

	// Renders as a table because a grid view is one, and nests partsTable so its
	// rows carry over.
	view fleetTable : GridView {
		expose Fleet::Engine;
		view partsSubtable :> partsTable;
	}
}
