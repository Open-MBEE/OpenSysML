// An operation over an empty collection answers the empty collection without
// ever calling its body — there is no element to call it with — so a body that
// would fail on an element is not a failure here.
package test {
	calc emptyOps {
		attribute xs : Integer[*] = ();
		attribute mapped : Integer[*] = xs.{in x; x * 2};
		attribute kept : Integer[*] = xs.?{in x; x > 0};
		return : Integer = SequenceFunctions::size(mapped) * 10
			+ SequenceFunctions::size(kept)
			+ NumericalFunctions::sum(mapped);
	}
}
