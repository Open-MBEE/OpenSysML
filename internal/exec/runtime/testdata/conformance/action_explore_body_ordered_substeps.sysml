package test {
	private import ScalarValues::*;

	action def Ordered {
		attribute log : String := "";
		// Oracle (docs/project/behavior-semantic-oracle.md): a's two substeps are
		// ordered by a succession, so b may run before, between or after them, never
		// with the second before the first.
		first start;
		then fork f;
			then a;
			then b;
		action a {
			first start then a1;
			action a1 { assign log := log + "1"; }
			then a2;
			action a2 { assign log := log + "2"; }
			then done;
		}
		action b { assign log := log + "b"; }
		succession a then j;
		succession b then j;
		join j;
		then done;
	}
}
