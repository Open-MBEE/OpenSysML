// A parameter states a relationship either with an operator or with the keyword
// the operator stands for.
package ParameterRelationshipKeywords {
	part def Wheel;
	action def Measure {
		in part w subsets Wheel;
		out mass specializes Wheel;
		in ref r references Wheel;
		out part hub : Wheel;
		out part rim :> Wheel;
	}
}
