// Elements whose fixed values meet the harder cases: an assumption alongside a
// required condition, a Natural domain with a division whose divisor is guarded,
// and a usage denying a constraint.
package test {
	private import ScalarValues::*;
	public import SI::*;
	public import ISQ::*;

	requirement def Landing {
		attribute actual : ISQSpaceTime::SpeedValue = 3.0 [m/s];
		attribute limit : ISQSpaceTime::SpeedValue = 5.4 [km/h];
		assume constraint {
			limit <= 5.4 [km/h]
		}
		require constraint {
			actual <= limit
		}
	}

	part def Rig {
		attribute total : Natural = 7;
		attribute shares : Natural;

		assert constraint sharing {
			total / shares >= 2
		}
	}

	constraint def Wide {
		in width : Integer;
		assert constraint {
			width >= 20
		}
	}

	part frame {
		assert not constraint notWide : Wide;
	}
}
