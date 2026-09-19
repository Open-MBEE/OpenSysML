// The operations compose: a select of a collect of a select, and a collect over
// an empty collection, which maps to an empty result without the body running.
package test {
	calc nested {
		attribute xs : Integer[*] = (1, 2, 3, 4);
		attribute empty : Integer[*] = ();
		attribute odds : Integer[*] = xs.?{in x; x == 1 or x == 3};
		attribute squares : Integer[*] = odds.{in x; x * x};
		attribute big : Integer[*] = squares.?{in x; x > 1};
		attribute none : Integer[*] = empty.{in x; x * 2}.?{in x; x > 0};
		return : Integer = SequenceFunctions::size(none) * 100 + big#(1) + SequenceFunctions::size(squares);
	}
}
