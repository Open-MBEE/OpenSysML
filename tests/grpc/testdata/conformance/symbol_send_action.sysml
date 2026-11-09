// A send action read as a symbol: its kind is the send, and its metadata states
// the payload, the `via` sender and the `to` receiver as written.
package SendActions {
	item def Alarm {
		attribute level : ScalarValues::Integer;
	}
	port def AlarmPort {
		out item alarm : Alarm;
	}
	part def Monitor {
		port alarmPort : AlarmPort;
		part display;

		action supervise {
			action notifyVia send new Alarm(3) via alarmPort;
			action notifyTo send new Alarm(5) to display;
		}
	}
}
