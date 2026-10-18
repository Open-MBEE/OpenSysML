package RedefiningHeads {
	private import Views::*;

	part def A;
	part def B;
	package Q {
		part def A;
	}

	part def Base {
		part left : A;
		part right : B;
		part single : A;
		part one : A;
		part other : A;
		part lb : A, B;
		part rb : B;
		part ql : Q::A;
		part qr : A;
	}

	// A usage redefining two typed features inherits both types; one redefining
	// a chain of untyped redefinitions reaches the type at its end.
	part def Derived :> Base {
		part both :>> left, right;
		part :>> single;
		part pair :>> one, other;
		part lbrb :>> lb, rb;
		part q :>> ql, qr;
	}

	part def Further :> Derived {
		part :>> both;
		part :>> single;
		part :>> pair;
		part :>> lbrb;
		part :>> q;
	}

	part d : Further;

	view heads {
		expose d;
		attribute depth = 2;
		render asInterconnectionDiagram;
	}

	view headsTree {
		expose Further;
		render asTreeDiagram;
	}

	view headsTable {
		expose Further;
		render asElementTable;
	}
}
