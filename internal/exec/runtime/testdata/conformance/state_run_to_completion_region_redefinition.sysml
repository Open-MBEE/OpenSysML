// A substate of an orthogonal region carries a non-run-to-completion
// redefinition; the region content executes through the same lowered state path.
package test {
	attribute def Go;

	part def Controller {
		exhibit state mode parallel {
			state left {
				entry; then l1;
				state l1 {
					attribute :>> isRunToCompletion = false;
				}
			}
			state right {
				entry; then r1;
				state r1;
			}
		}
	}

	part ctl : Controller;
}
