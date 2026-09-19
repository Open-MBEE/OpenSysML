// A collection operation is reached three ways — the qualified call, the
// unqualified name, and the receiver form `col->op(…)` — and all three name one
// implementation, so they answer alike.
package test {
	private import SequenceFunctions::*;
	calc agree {
		attribute xs : Integer[*] = (1, 2, 3);
		attribute qualified : Integer = SequenceFunctions::size(xs);
		attribute unqualified : Integer = size(xs);
		attribute receiver : Integer = xs->size();
		return : Integer = if qualified == unqualified and unqualified == receiver ? receiver else 0 - 1;
	}
}
