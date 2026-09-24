// KerML conjugation relates any two Types (KerML 1.1 §7.4; KerML.xtext
// ClassifierConjugationPart, FeatureConjugationPart, Conjugation), so none of
// these declarations names a port definition. The shapes are those of
// kerml-examples `Simple Tests/Conjugation.kerml`, `Types.kerml`, `Features.kerml`,
// on which the pinned validate-kerml is silent.
package F90Conjugation {
	class A {
		in feature f;
	}

	class B conjugates A;
	type C ~ A;
	feature g ~ B::f;

	type Original specializes Base::Anything {
		in feature Input;
	}
	type Conjugate1 specializes Base::Anything;
	type Conjugate2 specializes Base::Anything;
	conjugation c1 conjugate Conjugate1 conjugates Original;
	conjugation c2 conjugate Conjugate2 ~ Original;

	type Conjugate3 conjugates Original;
	type Conjugate4 ~ Conjugate1;

	classifier Tanks {
		feature fuelInPort {
			in feature fuelFlow;
		}
		feature fuelOutPort ~ fuelInPort;
	}
}
