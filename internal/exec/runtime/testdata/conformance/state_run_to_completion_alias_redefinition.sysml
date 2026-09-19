// A redefinition names the library feature through an alias: lowering resolves
// the target to Occurrences::Occurrence::isRunToCompletion and refuses the
// machine like one redefining the feature by its own name.
package test {
	attribute def Go;
	alias Rtc for Occurrences::Occurrence::isRunToCompletion;

	part def Controller {
		exhibit state mode {
			attribute :>> Rtc = false;
			entry; then idle;
			state idle;
			accept Go then done;
			state done;
		}
	}

	part ctl : Controller;
}
