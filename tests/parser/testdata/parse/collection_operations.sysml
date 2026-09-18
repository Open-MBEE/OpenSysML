// The collection operations and the three syntaxes they are written in: the
// collect notation `.{…}` and the select notation `.?{…}`, both taking a body
// expression whose `in` parameters the body declares; the receiver form
// `col->op(…)`, whose argument may itself be a body; and the plain call.
package collections {
	attribute xs = (1, 2, 3);

	attribute doubled = xs.{in x; x * 2};
	attribute large = xs.?{in x; x > 1};
	attribute nested = xs.{in x; xs.?{in y; y > x}};

	attribute count = xs->size();
	attribute kept = xs->select {in x; x > 1};
	attribute joined = xs->union((4, 5));
	attribute total = SequenceFunctions::size(xs);
	attribute third = xs#(3);

	// A body parameter may declare a type, a multiplicity and a specialization,
	// which belong to the parameter rather than to the body's result.
	attribute typed = xs.{in x : Integer[1] :> ISQ::mass; x};
}
