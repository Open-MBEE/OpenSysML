// `this` in the body of an action usage a part definition owns is the part
// performing it, not the action performance itself: the usage is one of the
// part's owned performances, whose `this` defaults to the owning object
// (`Objects::Object::ownedPerformances` redefines `this default that`). So
// redefining the action's `ref part ctx : P` with `= this` holds the owning
// part, read as a materialized feature value without the action ever running.
package test {
	action def A {
		ref part ctx : P;
	}

	part def P {
		action a : A {
			:>> ctx = this;
		}
	}

	part p : P;
}
