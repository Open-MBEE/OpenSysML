// A state declaring no substate performs its entry action and stays active,
// however it is typed: no initial state is asked of it (SysML v2 §7.18.1).
package test {
	private import ScalarValues::*;
	private import States::StateAction;

	state def Armed :> StateAction;

	part def Mission {
		attribute armed : Integer = 0;
		attribute launched : Integer = 0;
		attribute idled : Integer = 0;

		exhibit state arming : Armed {
			entry action arm {
				assign armed := armed + 1;
			}
		}
		exhibit state phases : StateAction {
			entry action mark {
				assign launched := launched + 1;
			}
		}
		exhibit state idle {
			entry action rest {
				assign idled := idled + 1;
			}
		}
	}
}
