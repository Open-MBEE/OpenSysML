// An exhibited machine typed by the library's StateAction with no body of its
// own takes nothing from the library definition, whose self-typed `self` state
// is withheld: the machine lowers and fails only for stating no initial state.
package test {
	private import States::StateAction;

	part def Mission {
		attribute mass = 12;

		exhibit state phases : StateAction;
	}
}
