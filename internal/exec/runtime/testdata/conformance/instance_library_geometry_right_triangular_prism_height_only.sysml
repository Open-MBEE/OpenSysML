// A RightTriangularPrism given only its height leaves length and width, and every face and
// edge dimension derived from them, without a value: the check `-instantiate` makes reports
// none of those, as it reports no unset attribute, and reading one is the typed no-value
// error. The library's self-bound `rf.width` is still reported, once.
package test {
	private import ShapeItems::*;
	private import SI::*;

	part def Wedge {
		item :>> shape : RightTriangularPrism {
			attribute :>> height = 1 [m];
		}
	}

	part wedge : Wedge;
}
