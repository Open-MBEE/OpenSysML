// A qualified name whose segments cross an import resolves at evaluation as the
// checker resolves it: a package that publicly imports another re-exports its
// members, at any depth of façade and by short name or alias as well.
package A {
	private import ScalarValues::*;
	attribute x : Integer = 1;
	attribute <s> shortNamed : Integer = 2;
	package Deep { attribute d : Integer = 3; }
}
package Wild { public import A::*; }
package Single { public import A::x; }
package Facade { public import Wild::*; }
package Recursive { public import A::**; }
package Aliased { alias ax for A::x; }
package test {
	private import ScalarValues::*;

	calc total {
		return : Integer = Wild::x + Single::x + Facade::x + Facade::Deep::d
			+ Recursive::d + Aliased::ax + Wild::s;
	}
}
