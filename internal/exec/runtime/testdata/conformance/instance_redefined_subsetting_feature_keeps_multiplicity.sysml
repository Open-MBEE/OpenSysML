// A feature redefining a valued subsetting feature and restating no multiplicity is bound as it is
// (KerML 1.0 §7.3.4.5): by `vertices [*]`, whether it binds its own value or keeps the redefined one.
package test {
	item def Point;
	item def Face {
		item corners : Point [2];
	}
	item def Wedge {
		item faces : Face [*];
		item left [1] :> faces;
		item right [1] :> faces;
		item vertices : Point [*] = faces.corners;
		item ridge :> vertices default (left.corners, right.corners);
	}
	item def Roof :> Wedge {
		item :>> ridge = (right.corners, left.corners);
	}
	item def Gable :> Roof {
		item :>> ridge;
	}
}
