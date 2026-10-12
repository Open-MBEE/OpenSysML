// A SequenceExpression admits a trailing comma (KerMLExpressions.xtext
// SequenceExpression: `OwnedExpression (',' | operator=',' operand+=SequenceExpression)?`),
// which adds no operand: `(a,)` is `a`, and `(a, b,)` is the pair.
package Trailing {
	attribute a : ScalarValues::Integer = 1;
	attribute b : ScalarValues::Integer = 2;
	attribute pair : ScalarValues::Integer[*] = (
		a,
		b,
	);
	attribute one : ScalarValues::Integer[*] = (a,);
	attribute plain : ScalarValues::Integer[*] = (a);
	attribute second : ScalarValues::Integer = pair#(2,);
}
