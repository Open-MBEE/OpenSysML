// Constraints whose feature set decides the logic: integer division by a literal
// stays linear, a variable divisor is nonlinear, and reading an Int and a Real
// together needs a mixed logic, linear or not.
package test {
	private import ScalarValues::Integer;
	private import ScalarValues::Real;

	constraint def CratesPerPallet {
		in crates : Integer;
		assert constraint {
			crates / 12 >= 3
		}
	}

	constraint def CratesPerRun {
		in crates : Integer;
		in runs : Integer;
		assert constraint {
			runs > 0
			crates / runs >= 3
		}
	}

	constraint def MassAndCrates {
		in crates : Integer;
		in mass : Real;
		assert constraint {
			mass <= 4.5
			crates >= 2
		}
	}

	constraint def MassPerCrate {
		in crates : Integer;
		in mass : Real;
		assert constraint {
			crates > 0
			mass / crates <= 4.5
		}
	}
}
