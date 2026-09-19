// An index naming no position of the sequence is a diagnostic, not an empty
// result: answering nothing for `xs#(4)` would let a model compute with a
// missing value it never asked for.
package test {
	calc pick {
		attribute xs : Integer[*] = (10, 20, 30);
		return : Integer = xs#(4);
	}
}
