// A performed action's pins cross the wire in the shape their multiplicity gives them:
// a [1] pin is its one value, a pin written without a multiplicity ([0..*]) is a
// one-element sequence, and a [0..*] pin holding several values is that sequence.
package test {
	private import ScalarValues::*;

	action def Shape {
		in value : Integer[1];
		out one : Integer[1];
		out bare : Integer;
		out many : Integer[0..*];
		first start;
		then action s { assign one := value; assign bare := value; assign many := (value, value + 1); }
		then done;
	}
}
