// An `assert constraint` a succession orders among a case's steps is judged
// where the flow reaches it (SysML v2 §7.20.3): `filled` and `not overfull`
// read the level `fill` left, which `drain` lowers only afterwards.
package test {
	private import ScalarValues::*;

	analysis tank {
		attribute level : Integer := 0;
		action fill { assign level := 3; }
		then assert constraint filled { level == 3 }
		then assert not constraint overfull { level > 3 }
		then action drain { assign level := 1; }
		return remaining : Integer = level;
	}
}
