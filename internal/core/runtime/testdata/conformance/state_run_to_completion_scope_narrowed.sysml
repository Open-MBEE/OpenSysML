// A substate redefining runToCompletionScope to itself narrows the scope from
// the whole machine (Occurrences::Occurrence::runToCompletionScope default
// self); the runtime implements only the whole-machine scope and refuses the
// narrowing with a typed error naming the state and the scope written.
package test {
	attribute def Go;

	part def Controller {
		exhibit state mode {
			entry; then idle;
			state idle {
				ref :>> runToCompletionScope = self;
			}
			accept Go then done;
			state done;
		}
	}

	part ctl : Controller;
}
