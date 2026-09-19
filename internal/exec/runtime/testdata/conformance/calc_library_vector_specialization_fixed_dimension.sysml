// An object of a specialization whose dimension the library fixes
// (`ThreeVectorValue::dimension = 3`) is the three-vector its elements shape:
// its own members answer, and its `dimension` is the one the type fixes.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import VectorFunctions::*;

	attribute def Labeled3 :> CartesianThreeVectorValue {
		attribute label : String = "three";
	}

	attribute v : Labeled3 {
		:>> elements = (1.0, 2.0, 2.0);
	}

	calc def probe {
		return : Base::Anything[3] nonunique = (v.label, v.dimension, norm(v));
	}
}
