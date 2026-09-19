package Vehicles {
	part def Wheel;

	part def Engine {
		attribute power;
		part cylinder[4] : Cylinder;
	}

	part def Cylinder;

	part def Vehicle {
		part engine : Engine;
		part wheels[4] : Wheel;
		connect engine to wheels;
	}
}

package VehicleViews {
	private import Views::*;

	view engineView {
		expose Vehicles::Engine;
	}

	// States no rendering, so the default tree renders it, and nests engineView
	// as a subtree.
	view vehicleView {
		expose Vehicles::Vehicle;
		view engineSubview :> engineView;
	}
}
