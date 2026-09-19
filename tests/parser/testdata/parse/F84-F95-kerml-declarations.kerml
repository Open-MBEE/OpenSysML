package KerMLDeclarations {
	private import ScalarValues::*;

	metaclass Classified;

	classifier A specializes Base::Anything;
	classifier B specializes Base::Anything;

	// F84: features declared with no kind keyword.
	a : Integer;
	x;
	p5[1] : Real;

	// F85: `type`, whose specialization is mandatory.
	type T specializes A;
	type all u specializes A, Base::things;

	// F86: relationships written keyword-first.
	feature f;
	feature g;
	specialization Gen subtype A specializes B;
	specialization t1 typing f typed by B;
	specialization Sub subset f subsets g;
	featuring F of f by A;
	inverting i inverse f of g;

	// F87 and F90: the long typing spelling, and conjugation in a declaration.
	feature x1 typed by A, B subsets g;
	classifier C conjugates A;

	classifier Tanks specializes Base::Anything {
		feature fuelInPort {
			in feature fuelFlow : Real;
		}
		feature fuelOutPort ~ fuelInPort;
	}

	class D {
		feature a : D;
		feature b : D;
		feature c : D;
		feature vin;

		// F88 and F89: chained connector ends, and optional binding declarations.
		connector c.a to a.b;
		binding {
			end feature references a;
			end feature references b;
		}
		succession {
			end feature references a;
			end feature references b;
		}

		// F84: prefixed features.
		composite var identifier[0..1];
		const q : Real;
	}

	// F86: a redefinition stated keyword-first.
	feature legalIdentification;
	redefinition D::vin redefines legalIdentification;

	// F91: `const` before `end`.
	assoc struct AS {
		const end [1] feature e1;
		const end feature e2;
	}

	// F92: an anonymous locale comment, and `doc` with a short name.
	comment locale "en" /* an anonymous comment */
	doc <d> /* documentation with a short name */

	// F93 and F94: repeated import filters, and prefix metadata as the keyword.
	private import KerMLDeclarations::**[@Classified][true];
	abstract #Classified z2;

	// F95: a named expression whose body needs no ';'.
	behavior W {
		inout v : Integer;
		expr whileTest {v > 3}
	}
}
