// `terminate <feature chain>;` naming an object that is no node of the action's
// flow ends that object (SysML v2 §7.17.10): the machine it exhibits is
// terminated where it stands, its do behavior abandoned, while the action stating
// the terminate runs on to its own end.
package test {
	private import ScalarValues::*;

	attribute def Never;

	state def Beating {
		attribute beats : Integer = 0;
		entry; then alive;
		state alive {
			do action beat {
				first start;
				then action one assign beats := 1;
				then action wait accept Never;
				then action two assign beats := 2;
				then done;
			}
		}
	}

	part def Heart {
		exhibit state rhythm : Beating;
	}

	part def Body {
		part heart : Heart;
	}

	part patient : Body;

	action host {
		out attribute before : Integer = 0;
		out attribute after : Integer = 0;
		first start;
		then action check assign before := patient.heart.rhythm.beats;
		then action stop { terminate patient.heart; }
		then action tail assign after := patient.heart.rhythm.beats;
		then done;
	}
}
