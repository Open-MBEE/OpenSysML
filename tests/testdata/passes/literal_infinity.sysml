// `*` written as a value is a LiteralInfinity, whose result is a Positive (KerML
// checkLiteralInfinitySpecialization), so it binds to every numeric scalar type
// and to nothing else. The pinned pilot validator warns "Bound features should
// have conforming types" on exactly b, s, p and lvl below and is silent on the
// rest; the value-count rule on `two` is this checker's own, as it is for `= 1`.
package LiteralInfinity {
	private import ScalarValues::*;
	part def PD;
	enum def Level { high; low; }
	attribute b : Boolean = *;
	attribute s : String = *;
	part p : PD = *;
	attribute lvl : Level = *;
	attribute n : Natural = *;
	attribute m : Positive = *;
	attribute i : Integer = *;
	attribute q : Rational = *;
	attribute r : Real = *;
	attribute ng : Integer = -*;
	attribute cmp : Boolean = * == 1;
	attribute pair : Integer[2] = (1, *);
	attribute two : Integer[2] = *;
}
