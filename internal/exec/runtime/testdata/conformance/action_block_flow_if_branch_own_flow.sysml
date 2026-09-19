package test {
	private import ScalarValues::*;

	// An action declared in an `if` branch may state a flow of its own, a fork and
	// join included: the branch performs it to completion before the sibling that
	// reads its pins runs. Only the taken branch's node is performed.
	action outer {
		in attribute mode : Integer = 1;
		out attribute total : Integer = 0;

		first start;
		then action choose {
			if mode == 1 {
				action split {
					out sum : Integer;
					first start;
					then fork f;
						then left;
						then right;
					action left { out a : Integer; assign a := 10; }
					action right { out b : Integer; assign b := 20; }
					succession left then j;
					succession right then j;
					join j;
					then action gather { assign sum := left.a + right.b; }
					then done;
				}
				action report { assign total := split.sum; }
			} else {
				action split { out sum : Integer; assign sum := 0; }
				action report { assign total := split.sum - 1; }
			}
		}
		then done;
	}
}
