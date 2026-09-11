// A KerML sequence is flat, so writing two collections side by side is the same
// sequence as their union — which is how the library defines union, as the
// sequence expression `(seq1, seq2)`. The two spellings must therefore agree on
// their size, their elements and what each position holds.
package test {
	private import SequenceFunctions::*;
	calc flat {
		attribute xs : Integer[*] = (1, 2, 3);
		attribute ys : Integer[*] = (2, 4);
		attribute written : Integer[*] nonunique = (xs, ys);
		attribute joined : Integer[*] nonunique = xs->union(ys);
		attribute mapped : Integer[*] nonunique = xs.{in x; (x, x)};
		return : Integer =
			if written->equals(joined) and written->size() == 5 and written#(4) == 2 and mapped->size() == 6
				? written->size() + mapped->size()
				else 0 - 1;
	}
}
