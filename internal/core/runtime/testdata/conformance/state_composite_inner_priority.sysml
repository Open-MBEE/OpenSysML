// Both the substate and the composite state enclosing it react to Abort: the
// innermost enabled transition wins, so Step1 moves to Step2 and Working is not
// left (KerML StatePerformances::StatePerformance::acceptable, the isDispatch
// invariant: an enclosing state accepts a transfer only if no substate did).
package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working {
			exit { assign log := log * 10 + 9; }
			state Step1 {
				exit { assign log := log * 10 + 1; }
			}
			state Step2 {
				entry { assign log := log * 10 + 2; }
			}

			transition first Step1 when Abort then Step2;
		}
		state Done {
			entry { assign log := log * 10 + 9; }
		}

		succession first start then Step1;
		transition first Working when Abort then Done;
	}
}
