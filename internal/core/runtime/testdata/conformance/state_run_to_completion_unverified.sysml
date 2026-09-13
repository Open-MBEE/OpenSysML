// A redefinition whose value lowering cannot read as the library default is
// refused as unverified rather than assumed to restate it: the message says the
// runtime cannot verify the default.
package test {
	private import ScalarValues::*;

	attribute def Go;

	part def Controller {
		attribute strict : Boolean = true;
		exhibit state mode {
			attribute :>> isRunToCompletion = strict or true;
			entry; then idle;
			state idle;
			accept Go then done;
			state done;
		}
	}

	part ctl : Controller;
}
