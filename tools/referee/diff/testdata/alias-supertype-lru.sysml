// ShapeItems::Box is `alias Box for RectangularCuboid`, so redefining `length`
// through it only resolves if aliases are followed through type relationships.
package 'Alias Supertype Probe' {
	private import ShapeItems::Box;
	private import SI::*;

	part def AvionicsLRU :> Box {
		:>> length = 100 [mm];
	}
}
