// The eleven words of docs/reference/grammar/conformance-audit.md are a literal
// in none of the pinned grammars, so each is an ordinary name here. `done` is one
// in the normative library (`Systems Library/Actions.sysml`).
package P {
	part def Vehicle {
		attribute done : Boolean;
		attribute final : Boolean;
		attribute initial : Boolean;
		attribute region : Boolean;
		attribute choice : Boolean;
		attribute junction : Boolean;
		attribute history : Boolean;
		attribute deep : Boolean;
		attribute shallow : Boolean;
		attribute defer : Boolean;
		attribute decision : Boolean;

		attribute anyDone = done or final;
		attribute depth = if deep ? 2 else 1;
	}

	part def Journey;
	item def done;
	part def region {
		part history;
	}

	action Report {
		in done : Boolean;
		out decision : Boolean;
		assign decision := done;
	}
}
