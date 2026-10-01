// A nonliteral redefinition is evaluated in its lowered scope; this expression
// resolves to true and proves dynamic values do not add a literal-evaluation trace.
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
