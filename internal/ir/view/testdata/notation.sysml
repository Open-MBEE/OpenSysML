// Declarations whose written keyword differs from the kind they classify as, rendered by notation.
// Deliberately invalid in one place: `behavior` is a KerML keyword the SysML grammar has no production for; the error is kept, as the fixture pins that a rendering names the declaration as it was written.
package Notation {
	behavior def BD;
	part def Wheel;
	part 'my wheel' : Wheel;
	action def Run {
		assert constraint moving { true }
		perform action drive;
	}
}
