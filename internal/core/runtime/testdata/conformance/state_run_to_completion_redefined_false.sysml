// A machine redefining isRunToCompletion to false asks for a scheduling the
// library gives no semantics to (Occurrences::Occurrence::isRunToCompletion
// default true): lowering refuses it with a typed error naming the feature,
// the declaring body and the value written, instead of running it under the default.
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
