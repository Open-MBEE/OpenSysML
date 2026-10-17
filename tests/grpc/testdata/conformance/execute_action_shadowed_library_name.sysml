// A model type named like a standard-library package shadows it, as the
// notation resolves names, so an action it inherits is reached through it.
part def Base {
	private import ScalarValues::*;

	action addFive {
		attribute result : Integer = 0;
		first start;
		action inner {
			assign result := result + 5;
		}
		done;
		succession first start then inner;
		succession first inner then done;
	}
}

part def Items :> Base;
