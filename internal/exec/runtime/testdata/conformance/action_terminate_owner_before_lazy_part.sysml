// A part of an object exists as long as its whole does (SysML v2 §7.17.10): when
// `terminate` ends the whole before an owned part is first read, the read still
// answers the part's declared values but reaches an object that ended with its
// whole — no behavior of it starts, so the engine's machine never fires.
package test {
	private import ScalarValues::*;

	part def Engine {
		attribute rate : Integer = 60;
		attribute fired : Integer = 0;
		exhibit state life {
			entry; then on;
			state on { entry action ignite assign fired := 1; }
		}
	}

	part def Body {
		// Optional, so nothing reads it when the body is created; the binding
		// makes it hold one engine once read.
		part engine : Engine[0..1] { :>> rate = 50; }
	}

	part patient : Body;

	action host {
		out attribute rate : Integer = 0;
		out attribute fired : Integer = 0;
		first start;
		then action stop { terminate patient; }
		then action read assign rate := patient.engine.rate;
		then action check assign fired := patient.engine.fired;
		then done;
	}
}
