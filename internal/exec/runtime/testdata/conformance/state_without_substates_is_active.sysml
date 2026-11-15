// A state machine declaring no substate is active in itself: its entry action
// is performed once and no initial state is asked of it (SysML v2 §7.18.1).
package test {
	private import ScalarValues::*;

	state def Standby {
		attribute entered : Integer = 0;
		entry action note {
			assign entered := entered + 1;
		}
	}
}
