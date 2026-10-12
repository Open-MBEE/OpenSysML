// The trailing comma a SequenceExpression admits (KerMLExpressions.xtext
// SequenceExpression) is admitted wherever the grammar reuses the production:
// a parenthesized sequence, a `#(…)` index and a `[…]` index.
package Trailing {
	feature arr : Collections::Array;
	feature pair = (1, 2,);
	feature one = (1,);
	feature arr13 = arr#(1, 3,);
	feature elem = arr[1, 3,];
}
