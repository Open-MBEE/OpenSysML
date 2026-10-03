action def Objects {
	first start then produce;
	action produce { out y : Real; }
	fork split {
		in ref inputObject1 : Real;
		out ref outputObject1 : Real = inputObject1;
		out ref outputObject2 : Real = inputObject1;
	}
	join pair {
		in ref inputObject1 : Real;
		in ref inputObject2 : Real;
		out ref outputObject1 : Real nonunique = (inputObject1, inputObject2);
	}
	merge pick {
		in ref inputObject1 : Real;
		out ref outputObject1 : Real = (inputObject1);
	}
	action keep { in v : Real[2]; }
	succession flow of Real from produce.y to split.inputObject1;
	succession flow of Real from split.outputObject1 to pair.inputObject1;
	succession flow of Real from split.outputObject2 to pick.inputObject1;
	succession flow of Real from pick.outputObject1 to pair.inputObject2;
	succession flow of Real from pair.outputObject1 to keep.v;
}
