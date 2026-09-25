// The payload an accept node binds is the argument of a sibling typed action:
// `in level = msg.level` resolves msg through the enclosing action body, the
// way a nested if or loop body does, and the typed action computes from it.
package test {
	private import ScalarValues::*;

	item def Warning { attribute level : Real; }

	action def Handle {
		in level : Real;
		out doubled : Real;
		first start then compute;
		action compute {
			in x : Real;
			out result = x * 2.0;
		}
		bind compute.x = level;
		bind doubled = compute.result;
		first compute then done;
	}

	action watcher {
		attribute total : Real = 0.0;

		first start;

		action sender {
			send new Warning(level = 2.5) to receiver;
		}

		action receiver accept msg : Warning;

		action run : Handle { in level = msg.level; }

		action keep {
			assign total := run.doubled;
		}

		done;

		succession first start then sender;
		succession first sender then receiver;
		succession first receiver then run;
		succession first run then keep;
		succession first keep then done;
	}
}
