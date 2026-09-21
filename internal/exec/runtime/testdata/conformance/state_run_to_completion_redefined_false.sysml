// A machine redefining isRunToCompletion to false opts out of the default
// entry boundary, so the lowered runtime executes the machine under that
// declared scheduling behavior.
package test {
	attribute def Go;

	part def Controller {
		exhibit state mode {
			attribute :>> isRunToCompletion = false;
			entry; then idle;
			state idle;
			accept Go then done;
			state done;
		}
	}

	part ctl : Controller;
}
