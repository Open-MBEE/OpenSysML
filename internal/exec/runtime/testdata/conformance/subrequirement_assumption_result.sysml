// The container's `result` reads a subrequirement as one constraint whose value is its own `assumptions implies constraints`: rFails.result is false because R's
// own constraint fails whatever child assumes; rVacuous.result is true, its only constraint being a subrequirement whose assumption is false.
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	requirement def R {
		subject v : Veh;
		require constraint { v.mass < 10.0 }
		requirement child {
			assume constraint { false }
			require constraint { false }
		}
	}
	requirement def V {
		subject v : Veh;
		requirement child {
			assume constraint { v.mass < 1.0 }
			require constraint { false }
		}
	}
	part veh : Veh { attribute :>> mass = 20.0; }
	part box {
		requirement rFails : R { subject v = veh; }
		requirement rVacuous : V { subject v = veh; }
	}
	attribute failsResult = box.rFails.result;
	attribute vacuousResult = box.rVacuous.result;
	calc probe { return one = 1; }
}
