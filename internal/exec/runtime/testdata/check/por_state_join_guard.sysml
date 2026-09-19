// An action's write to a feature a sibling join segment's guard reads, after
// sending the other segment its signal: whether the join fires on that signal
// depends on whether the write went first, so the dispatch is dependent on the
// write, and both are reached.
package test {
	private import ScalarValues::*;

	item def Go;

	part def Dial {
		attribute armed : Boolean = false;
	}

	part dial : Dial;

	action arm {
		first start;
		then action go { send Go() to gather; }
		then action set { assign dial.armed := true; }
		then done;
	}

	state gather parallel {
		attribute fired : Integer = 0;
		state left {
			entry; then l1;
			state l1;
			transition first l1 accept Go then sync;
		}
		state right {
			entry; then r1;
			state r1;
			transition first r1 if dial.armed then sync;
		}
		join sync;
		transition first sync do assign fired := 1 then done;
	}
}
