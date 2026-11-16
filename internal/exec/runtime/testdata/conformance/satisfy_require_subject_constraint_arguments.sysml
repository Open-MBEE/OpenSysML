// A requirement requiring its subject's constraint usage checks that usage with the
// arguments it states: `in demand = estimate` reads the subject's estimate.
package test {
	private import ScalarValues::*;

	constraint def Closes {
		in capability : Real;
		in demand : Real;
		capability >= demand * 1.25
	}
	part def Budget {
		attribute allotment : Real = 10.0;
		attribute estimate : Real default = 5.0;
		assert constraint closes : Closes { in capability = allotment; in demand = estimate; }
	}
	part budget : Budget;
	part overdrawn : Budget {
		attribute :>> estimate = 9.0;
	}
	requirement budgetCloses {
		subject b : Budget;
		require b.closes;
	}
	part context {
		assert satisfy budgetCloses by budget;
		assert not satisfy budgetCloses by overdrawn;
	}
}
