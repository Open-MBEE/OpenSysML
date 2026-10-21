// A RightTriangularPrism given its three dimensions — length, width and height — values
// every face and edge the library derives from them. Two declarations of the vendored
// library fail on their own terms and are reported once each: `rf`'s `width = rf.width` is
// bound to itself, and `hypotenuse`'s `length = (length^2 + width^2)` is an area written to
// a LengthValue. Nothing read through either is reported again.
package test {
	private import ShapeItems::*;
	private import SI::*;

	part def Wedge {
		item :>> shape : RightTriangularPrism {
			attribute :>> length = 3 [m];
			attribute :>> width = 4 [m];
			attribute :>> height = 1 [m];
		}
	}

	part wedge : Wedge;
}
