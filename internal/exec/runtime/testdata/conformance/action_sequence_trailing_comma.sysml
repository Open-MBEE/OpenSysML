// A sequence expression admits a trailing comma that adds no element
// (KerMLExpressions.xtext SequenceExpression): `(a, b,)` is the pair, and
// `(a,)` is `a` — a one-element sequence, exactly what `(a)` is — not a
// sequence holding a null.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	action def Trailing {
		attribute a : Integer = 1;
		attribute b : Integer = 2;
		out pair : Integer[*] = (a, b,);
		out one : Integer[*] = (a,);
		out plain : Integer[*] = (a);
		out second : Integer = pair#(2,);
		out count : Integer = (1, 2, 3,)->size();
	}
	action trailing : Trailing;
}
