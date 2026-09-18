// F99: declarations in an expression body are namespace members, including
// forward references and names visible to nested expression bodies.
package F99BodyDeclarations {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	calc def Evaluate {
		in values : Integer[*];
		values->collect {
			in i : Integer;
			private attribute computed : Integer = laterValue + 1;
			private attribute laterValue : Integer = i;
			computed
		}
	}
}
