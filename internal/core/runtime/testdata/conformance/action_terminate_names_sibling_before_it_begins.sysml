// The §7.17.10 MonitoredActivity shape: a forked branch terminates the performed
// action its sibling is, which the schedule has not begun yet. The sibling's step ends
// before it starts — its body never runs — and the succession out of it is taken.
package test {
	private import ScalarValues::*;

	attribute def TimeOut;

	action def MonitoredActivity {
		out attribute steps : Integer = 0;

		first start;
		then fork;
			then performCriticalActivity;
			then waitForTimeOut;
		action performCriticalActivity {
			first start;
			then action monitor accept TimeOut;
			then action critical assign steps := steps + 100;
			then terminate;
		}
		then stop;
		action waitForTimeOut {
			assign steps := steps + 1;
			terminate performCriticalActivity;
		}
		then stop;
		action stop terminate;
	}
}
