// A structural `first postSeparation then postFlyby;` is a succession over the
// two snapshots, so the first one stays a readable portion of the individual
// rather than being shadowed by an initial-node member of its own name.
package test {
	private import ScalarValues::*;

	individual part def Craft {
		attribute mass : Real;
	}

	individual part craft : Craft {
		attribute :>> mass = 120.0;

		snapshot postSeparation {
			attribute :>> mass = 118.0;
		}
		snapshot postFlyby {
			attribute :>> mass = 96.0;
		}
		first postSeparation then postFlyby;
	}
}
