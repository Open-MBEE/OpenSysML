// F71: `snapshot` and `timeslice` are SysML PortionKind literals (SysML.xtext:864)
// and no part of KerML, so in a KerML file they name features.
package Portions {
	class Interval;
	class Timeslice { feature interval : Interval; }
	class Occ {
		expr while {
			in timeslice : Timeslice;
			in snapshot : Timeslice;
			return result : Interval = timeslice.interval;
		}
	}
}
