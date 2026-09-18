// An index is a SequenceExpression, so a multi-dimensional index is one
// operand (KerMLExpressions.xtext PrimaryExpression `#` '(' SequenceExpression ')').
package Indexing {
	feature arr : Collections::Array;
	feature arr13 = arr#(1, 3);
	feature arr22 = arr#(2, 2);
	feature arr1 = arr#(1);
	feature elem = arr[1, 3];
}
