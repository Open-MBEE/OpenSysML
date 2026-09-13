// A substate of an orthogonal region carrying the redefinition is refused like
// any other state: region content is lowered through the same state path.
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
