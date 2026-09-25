// A transition's own guard reads a member of the payload its trigger bound
// through the transition's name, `raise.d.level`: the guard is evaluated within
// the transition's performance, after the accepter accepted the occurrence.
package StateGuardReadsOwnPayloadMember {
	private import ScalarValues::*;

	attribute def Reading {
		attribute level : Integer;
		attribute origin : String;
	}

	port def Link;

	part def Sensor {
		port out : Link;

		exhibit state report {
			entry; then sending;

			state sending {
				entry action emit {
					send new Reading(level = 9, origin = "probe") via out;
				}
			}
		}
	}

	part def Monitor {
		port in : Link;
		attribute origin : String = "";

		exhibit state watch {
			entry; then idle;

			state idle;
			state alerting;
			state ignored;

			transition raise first idle
				accept d : Reading via in
				if raise.d.level > 5
				do assign origin := raise.d.origin
				then alerting;

			transition first idle
				accept d : Reading via in
				if d.level <= 5
				then ignored;
		}
	}

	part def Rig {
		part sensor : Sensor;
		part monitor : Monitor;

		connect sensor.out to monitor.in;
	}

	part rig : Rig;
}
