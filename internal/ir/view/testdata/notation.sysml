// Declarations whose written keyword differs from the kind they classify as,
// so a rendering is checked against the notation rather than the classification.
package Notation {
	metaclass M;
	datatype T;
	feature f;
	class C;
	behavior def BD;
	part def Wheel;
	part 'my wheel' : Wheel;
	action def Run {
		assert constraint moving { true }
		perform action drive;
	}
}
