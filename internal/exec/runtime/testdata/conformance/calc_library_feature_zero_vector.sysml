// A vector-valued library feature reads as a name too, and the vector it
// answers is one CartesianThreeVectorValue; its elements read out as Reals.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;

	calc def displacement {
		return : CartesianThreeVectorValue = VectorFunctions::cartesian3DZeroVector;
	}

	calc def components {
		return : Real[3] = VectorFunctions::cartesian3DZeroVector.elements;
	}
}
