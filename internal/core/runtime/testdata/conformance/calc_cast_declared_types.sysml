// A value is kept by the type its feature is declared with as well as by the
// types its own content states: a custom scalar subtype keeps the values of the
// feature declared with it, and a scalar-valued enumeration keeps its literals
// (KerML 1.1 §8.3.4.9).
package test {
	private import ScalarValues::*;

	attribute def Even :> Integer;
	attribute def Counted :> Integer;

	enum def GradePoints :> Real {
		enum a = 4.0;
		enum b = 3.0;
	}

	attribute e : Even = 4;
	attribute g : GradePoints = GradePoints::a;
	attribute both : Even, Counted = 6;
	attribute evens : Even[0..*] = (2, 4, 6);

	calc def DeclaredCasts {
		out asDeclaredSubtype : Even[0..1] = e as Even;
		out asScalarSupertype : Integer[0..1] = e as Integer;
		out asOwnEnumeration : GradePoints[0..1] = g as GradePoints;
		out literalAsEnumeration : GradePoints[0..1] = GradePoints::a as GradePoints;
		out asEnumerationSupertype : Real[0..1] = g as Real;
		out asOtherScalar : Boolean[0..1] = e as Boolean;
		out asSecondDeclaredType : Counted[0..1] = both as Counted;
		out selectedAsSubtype : Even[0..*] = (evens.?{in x; x > 2}) as Even;
		out literalsAsEnumeration : GradePoints[0..*] = (GradePoints::a, GradePoints::b) as GradePoints;
		out mixedAsEnumeration : GradePoints[0..*] = (GradePoints::a, true) as GradePoints;
		out declaredWithLiteral : Even[0..*] = (e, false) as Even;
		out nestedAsEnumeration : GradePoints[0..*] =
			((GradePoints::a, GradePoints::b), GradePoints::a) as GradePoints;
		out multiValuedEntry : Even[0..*] = (evens, false) as Even;
	}

	calc c : DeclaredCasts;
}
