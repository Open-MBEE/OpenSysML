// A join's output object feature evaluates over the values its input object
// features hold once every incoming succession has arrived.
package test {
	private import ScalarValues::*;

	action gather {
		first start then both;
		fork both;
		first both then low;
		first both then high;
		action low { out y : Real; assign y := 1.5; }
		action high { out y : Real; assign y := 8.5; }
		join pair {
			in ref inputObject1 : Real;
			in ref inputObject2 : Real;
			out ref outputObject1 : Real nonunique = (inputObject1, inputObject2);
		}
		action keep { in v : Real[2]; }
		succession flow of Real from low.y to pair.inputObject1;
		succession flow of Real from high.y to pair.inputObject2;
		succession flow of Real from pair.outputObject1 to keep.v;
	}
}
