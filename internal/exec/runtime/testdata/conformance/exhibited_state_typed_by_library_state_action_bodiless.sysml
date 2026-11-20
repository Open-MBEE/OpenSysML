// A body-less machine typed by the library's StateAction takes no content from it:
// declaring no substate, it is active in itself (SysML v2 §7.18.1).
package test {
	private import States::StateAction;

	part def Mission {
		attribute mass = 12;

		exhibit state phases : StateAction;
	}
}
