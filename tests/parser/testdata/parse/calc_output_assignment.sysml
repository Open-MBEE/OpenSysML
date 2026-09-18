// Assignments to declared calc outputs in the body, beside a kind-less
// member `c = e;` that declares a feature rather than assigning one.
package Outputs {
	calc def TwoStmt {
		in n : Integer;
		out a : Integer;
		out b : Integer;
		assign a := n + 1;
		assign b := n + 2;
		c = n + 3;
	}
}
