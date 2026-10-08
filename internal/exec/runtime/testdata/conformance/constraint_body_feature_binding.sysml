package test {
	private import ScalarValues::*;

	// Constraint declarations are evaluated on each read in the current subject context.
	constraint def PositiveAfterUpdate {
		attribute twice : Integer = amount * 2;
		twice > 3
	}

	part def Subject {
		attribute amount : Integer := 0;

		action def Raise {
			first start;
			then action update { assign amount := 5; }
			then done;
		}

		perform action raise : Raise;
		constraint positive : PositiveAfterUpdate;
	}
}
