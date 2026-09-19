// A constraint whose body denies a group of conditions — `not (a and b)`, not
// `not a and not b` — and a usage asserting that the constraint itself does not
// hold, which denies the conjunction of everything it requires.
package test {
	private import ScalarValues::Integer;

	constraint def SafeWindow {
		in level : Integer;
		assert constraint {
			level >= 0
		}
		assert not constraint {
			level > 10
			level < 20
		}
	}

	part rig {
		assert not constraint safeWindow : SafeWindow;
	}
}
