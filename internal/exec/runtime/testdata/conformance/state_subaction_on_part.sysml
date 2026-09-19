// An entry, do or exit action given by a feature chain performs the action of
// the object the chain denotes, as that object: `exit controller.coolDown;`
// runs the controller's coolDown with the controller as `this`, so its own
// `cooled` flips while the machine's same-named attribute is untouched.
package test {
	private import ScalarValues::*;

	part def Controller {
		attribute cooled : Boolean default = false;

		action def CoolDown {
			first start then flip;
			action flip {
				assign this.cooled := true;
			}
			first flip then done;
		}

		action coolDown : CoolDown;
	}

	part def Machine {
		part controller : Controller;
		attribute cooled : Boolean default = false;

		exhibit state run {
			entry; then active;

			state active {
				exit controller.coolDown;
			}
			state cooled;

			transition first active if not controller.cooled then cooled;
		}
	}
}
