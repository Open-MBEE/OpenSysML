// An exhibited machine typed by the library's StateAction and stating its own
// body runs that body alone: the library definition's self-typed `self` state
// is withheld, so the object materializes and its attributes read.
package test {
	private import ScalarValues::*;
	private import States::StateAction;

	part def Mission {
		attribute mass : Integer = 12;
		attribute launched : Integer = 0;

		exhibit state phases : StateAction {
			entry; then launch;
			state launch {
				entry action mark {
					assign launched := 1;
				}
			}
		}
	}
}
