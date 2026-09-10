// A transition whose source is a composite state is enabled while any of its
// substates is active: the event is handled by the composite state, not by the
// substate it was offered to first (SysML v2 7.19.4; KerML
// StatePerformances::StatePerformance::acceptable over allSubstatePerformances).
package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working {
			exit { assign log := log * 10 + 2; }
			state Step1 {
				exit { assign log := log * 10 + 1; }
			}
			state Step2;

			transition first Step1 when Next then Step2;
		}
		state Done {
			entry { assign log := log * 10 + 4; }
		}

		succession first start then Step1;
		transition first Working when Abort do assign log := log * 10 + 3 then Done;
	}
}
