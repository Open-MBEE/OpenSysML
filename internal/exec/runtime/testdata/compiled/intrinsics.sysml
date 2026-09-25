// Calls of the library's scalar functions and reads of its scalar constants,
// reached by the unqualified name, the qualified name, an alias and the receiver
// form; and the names a model declares itself, which the library never answers.
package test {
	private import ScalarValues::*;
	private import RealFunctions::*;
	private import TrigFunctions::*;
	private import OpenSysMLMathFunctions::*;
	private import NumericalFunctions::*;
	private import IntegerFunctions::*;

	calc def Sqrt { in x : Real; return : Real = sqrt(x); }
	calc def Abs { in x : Real; return : Real = abs(x); }
	calc def Floor { in x : Real; return : Integer = floor(x); }
	calc def Round { in x : Real; return : Integer = round(x); }
	calc def Max { in x : Real; in y : Real; return : Real = max(x, y); }
	calc def Min { in x : Real; in y : Real; return : Real = min(x, y); }
	calc def IsZero { in x : Real; return : Boolean = isZero(x); }
	calc def IsUnit { in x : Real; return : Boolean = isUnit(x); }
	calc def Sin { in x : Real; return : Real = sin(x); }
	calc def Cos { in x : Real; return : Real = cos(x); }
	calc def Tan { in x : Real; return : Real = tan(x); }
	calc def Cot { in x : Real; return : Real = cot(x); }
	calc def Arcsin { in x : Real; return : Real = arcsin(x); }
	calc def Arccos { in x : Real; return : Real = arccos(x); }
	calc def Arctan { in x : Real; return : Real = arctan(x); }
	calc def Deg { in x : Real; return : Real = deg(x); }
	calc def Rad { in x : Real; return : Real = rad(x); }
	calc def Exp { in x : Real; return : Real = exp(x); }
	calc def Ln { in x : Real; return : Real = ln(x); }
	calc def Log { in x : Real; in b : Real; return : Real = log(x, b); }
	calc def Atan2 { in y : Real; in x : Real; return : Real = atan2(y, x); }
	calc def Pi { return : Real = pi; }
	calc def Sum { in x : Real; return : Real = sum(x); }
	calc def Product { in x : Real; return : Real = product(x); }

	// The Integer declarations, and the numeric ones keeping an Integer's kind.
	calc def IntAbs { in k : Integer; return : Integer = IntegerFunctions::abs(k); }
	calc def IntMax { in a : Integer; in b : Integer; return : Integer = IntegerFunctions::max(a, b); }
	calc def IntMin { in a : Integer; in b : Integer; return : Integer = IntegerFunctions::min(a, b); }
	calc def NumAbs { in k : Integer; return : Integer = NumericalFunctions::abs(k); }
	calc def NumMax { in a : Integer; in b : Integer; return : Integer = NumericalFunctions::max(a, b); }
	calc def IntSum { in k : Integer; return : Integer = IntegerFunctions::sum(k); }
	calc def IntProduct { in k : Integer; return : Integer = IntegerFunctions::product(k); }

	// The Real-only declarations refuse an Integer argument as the evaluator does.
	calc def RealSqrtOfInt { in k : Integer; return : Real = RealFunctions::sqrt(k); }
	calc def IntegerAbsOfReal { in x : Real; return : Integer = IntegerFunctions::abs(x); }

	// Qualified names reach the same declarations and the same constant.
	calc def Qualified {
		in x : Real;
		return : Real = RealFunctions::sqrt(x) * TrigFunctions::pi + OpenSysMLMathFunctions::exp(x);
	}

	// An alias for a library function is that function.
	alias root for RealFunctions::sqrt;
	calc def Aliased { in x : Real; return : Real = root(x); }

	// The receiver form passes the receiver as the first argument.
	calc def Receiver { in x : Real; in y : Real; return : Real = x->max(y) + x->sqrt(); }

	// Statement bodies compute with the functions and the constant.
	calc def Hypotenuse {
		in a : Real;
		in b : Real;
		attribute a2 = a * a;
		attribute b2 = b * b;
		attribute c = sqrt(a2 + b2);
		return : Real = c;
	}

	calc def Circumference {
		in r : Real;
		attribute tau = 2.0 * pi;
		return : Real = tau * r;
	}

	// Evaluator only: a function over strings, and a collection argument.
	calc def Length { in s : String; return : Integer = StringFunctions::Length(s); }
	calc def SumOfSequence { in k : Integer; return : Integer = sum((k, 1)); }
}

// A model declaring its own sqrt and pi: a call or read resolving to them is
// the model's, never the library's.
package shadowing {
	private import ScalarValues::*;
	private import RealFunctions::*;
	private import TrigFunctions::*;

	calc def sqrt { in x : Real; return : Real = x * 0.5; }
	attribute pi : Real = 3.0;

	calc def OwnSqrt { in x : Real; return : Real = sqrt(x); }
	calc def OwnPi { return : Real = pi; }
	calc def LibraryStill { in x : Real; return : Real = RealFunctions::sqrt(x) + TrigFunctions::pi; }
}
