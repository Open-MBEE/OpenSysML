// A KerML feature that declares and inherits no multiplicity has the usual
// default 0..* (KerML 1.1 §7.4.12, §7.4.5.2): a value of two is held as two,
// of one as one, of none as no value, and only a declared bound judges a count.
package test {
	private import ScalarValues::Real;

	class Sub {
		feature volume : Real = 2.0;
	}

	class Craft {
		feature left : Sub;
		feature right : Sub {
			feature :>> volume = 3.5;
		}
		feature both : Sub = (left, right);

		feature one : Real = left.volume;
		feature two : Real = both.volume;
		feature none : Real = ();
		feature bounded : Real[1] = both.volume;
	}

	feature craft : Craft;
}
