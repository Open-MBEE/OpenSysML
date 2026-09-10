// Inline entry, do and exit action bodies of a state that state a token flow of
// their own — `first start … then done`, `first a then b`, a fork and join, a
// decision with guards — as a standalone action body writes them.
package Flows {
	action def A;
	action def B;
	state def Machine {
		attribute n : Integer = 0;
		state s {
			entry action prep {
				first start;
				then action a : A;
				then action b : B;
				then done;
			}
			do action ops {
				action a : A;
				action b : B;
				first a then b;
			}
			exit action wrap {
				first start;
				then fork split;
				action a : A;
				action b : B;
				join sync;
				decide check;
				if n > 0 then c;
				else done;
				action c : B;
				succession first split then a;
				succession first split then b;
				succession first a then sync;
				succession first b then sync;
				succession first sync then check;
				succession first c then done;
			}
		}
		entry; then s;
		transition first s then done;
	}
}
