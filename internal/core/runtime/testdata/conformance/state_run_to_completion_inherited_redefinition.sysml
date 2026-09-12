// The redefinition sits on the state definition the exhibited machine is typed
// by, so the machine inherits it: lowering refuses the machine, naming the
// definition that declares the redefinition.
package test {
	attribute def Go;

	state def Base {
		attribute :>> isRunToCompletion = false;
		entry; then idle;
		state idle;
		accept Go then done;
		state done;
	}

	part def Controller {
		exhibit state mode : Base;
	}

	part ctl : Controller;
}
