// A named multiplicity states its bounds either as a range or by subsetting
// another multiplicity (KerML.xtext:754 MultiplicityRange, MultiplicitySubset).
package test {
	multiplicity zeroOrMore [0..*];
	multiplicity m subsets zeroOrMore;
	multiplicity n :> zeroOrMore {
		doc /* bounds by subsetting, with a body */
	}
	feature f {
		multiplicity [1..1];
	}
}
