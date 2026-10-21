// A valued feature declaring no multiplicity and subsetting a type-owned feature takes no default of
// one (SysML v2 §7.6.3) but its bound (KerML 1.0 §8.4.4.12.1): `ridge :> vertices` holds four corners.
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
		item ridge :> vertices = (left.corners, right.corners);
		item crest : Point [2] :> vertices = left.corners;
	}
}
