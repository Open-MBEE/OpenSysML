package test {
	// The bare `perform X;` shorthand: the statement is named after the action
	// it references (KerML Feature::effectiveName), so the succession names it
	// `increment`, while the reference itself must still resolve outward to the
	// action rather than to the statement.
	action outer {
		attribute base : Integer = 7;
		attribute result : Integer = 0;

		first start;

		perform increment;

		done;

		succession first start then increment;
		succession first increment then done;
	}

	action increment {
		in base : Integer;
		out result : Integer;

		first begin;

		action bump {
			assign result := base + 5;
		}

		done;

		succession first begin then bump;
		succession first bump then done;
	}
}
