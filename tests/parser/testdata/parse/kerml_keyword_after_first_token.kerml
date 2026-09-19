// KerML members whose second token is a kind keyword or a relationship word
// are prefixed declarations, a sufficient declaration, a binary connector's
// first end or a keyword-less feature (KerML.xtext FeaturePrefix, `all`,
// BinaryConnectorDeclaration, Feature): a name never stands ahead of the kind
// keyword, so `name keyword ...` is not a declaration.
package KerMLKeywordAfterFirstToken {
	class A;
	class Engine;
	class Tank;

	class Vehicle {
		feature all x : A;
		var feature v : A;
		composite feature c : A;
		derived feature d : A;
		#M feature m : A;
		feature eng : Engine;
		feature t : Tank;
		connector eng to t;
		connector [0..1] eng to [1..*] t;
		connector e ::> eng to t;
		y = 1;
		z : A = 2;
		w :> y;
		q [1] : A;
	}

	metaclass M;
	alias Motor for Engine;
	import Vehicle::*;
}
