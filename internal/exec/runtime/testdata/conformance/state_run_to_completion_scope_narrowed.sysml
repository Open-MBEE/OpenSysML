// A substate redefining runToCompletionScope to itself narrows the scope from
// the whole machine (Occurrences::Occurrence::runToCompletionScope default
// self); entry scheduling follows that lowered state-local scope.
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
