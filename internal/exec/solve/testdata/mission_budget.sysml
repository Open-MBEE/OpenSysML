// A constraint over the scalar value types: a Natural, which is not negative, a
// Boolean, a String compared for equality, and a conditional expression.
package test {
	private import ScalarValues::*;

	constraint def BudgetConstraint {
		in crewCount : Natural;
		in redundant : Boolean;
		in phase : String;
		assert constraint {
			(if redundant ? crewCount + 1 else crewCount) <= 4
		}
		assert constraint {
			phase == "descent" implies redundant
		}
	}
}
