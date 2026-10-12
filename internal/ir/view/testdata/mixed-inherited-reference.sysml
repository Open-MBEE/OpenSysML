package MixedInheritedReference {
	private import OpenSysMLRenderings::*;

	action def Drive;
	part def Person;
	state def Mode;

	use case def Base {
		subject task : Drive;
		actor driver : Person;
		perform task;
	}

	use case def A :> Base;
	use case def B :> Base;

	use case def ModeCase {
		subject mode : Mode;
	}

	use case def C :> ModeCase;
	use case def D :> ModeCase;

	part sysA {
		perform A::task;
	}

	part sysB {
		perform B::task;
	}

	part sysAll {
		perform Base::task;
	}

	part sysC {
		exhibit C::mode;
	}

	view mixedView {
		expose A;
		expose B;
		expose C;
		expose D;
		expose sysA;
		expose sysB;
		expose sysAll;
		expose sysC;
		expose Drive;
		expose Person;
		expose Mode;
		render asMixedDiagram;
	}
}
