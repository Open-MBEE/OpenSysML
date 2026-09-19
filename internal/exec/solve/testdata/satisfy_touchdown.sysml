// A satisfaction assertion: the requirement's conditions reach the values of its
// subject, and the assertion names the object supplying them.
package test {
	private import ScalarValues::Real;

	part def Lander {
		attribute verticalSpeed : Real;
	}

	requirement def TouchdownRequirement {
		subject craft : Lander;
		attribute maxVerticalSpeed : Real;
		require constraint {
			craft.verticalSpeed <= maxVerticalSpeed
		}
	}

	requirement touchdown : TouchdownRequirement;

	part lander : Lander {
		attribute :>> verticalSpeed = 1.2;
	}

	part context {
		assert satisfy touchdown by lander;
	}
}
