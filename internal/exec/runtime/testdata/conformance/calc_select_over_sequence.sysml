// `select` keeps the elements its predicate holds for, in the collection's
// order, and the notation `.?{…}` and the call agree because both evaluate
// through ControlFunctions::select.
package test {
	calc large {
		attribute xs : Integer[*] = (4, 12, 7, 30);
		attribute kept : Integer[*] = xs.?{in x; x > 10};
		attribute alsoKept : Integer[*] = ControlFunctions::select(xs, {in x; x > 10});
		return : Integer = SequenceFunctions::size(kept) * 100 + kept#(1) + alsoKept#(2);
	}
}
