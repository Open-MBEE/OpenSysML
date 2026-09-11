// An ordering operator is declared abstractly by DataFunctions and
// ScalarFunctions and concretely by the numeric libraries and StringFunctions
// alone (KerML 1.1 §9.3.4, §9.3.5, §9.3.6-§9.3.9, §9.3.12), so a value of
// any other type is refused as a type mismatch naming the operator, both
// operands and the library function that would have to declare it. An
// enumeration specializing Integer, Strings and quantities order as before.
package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	enum def Color { red; green; blue; }
	enum def Level :> Integer { low = 1; high = 3; }
	part def Widget;
	part widget : Widget;
	part other : Widget;
	metadata def Tag;
	#Tag part tagged : Widget;
	calc twice { in x : Integer; x * 2 }
	attribute widgets : Widget[*] = (widget, other);
	attribute nothing : Integer[0..1];

	calc def Orderings {
		out numericEnumeration : Boolean = Level::low < Level::high;
		out numericEnumerationReversed : Boolean = Level::high <= Level::low;
		out strings : Boolean = "a" < "b";
		out quantities : Boolean = 2 [m] > 100 [cm];
		out mixedNumbers : Boolean = 1 <= 2.5;
	}

	calc c : Orderings;

	attribute enumerationLess : Boolean = Color::red < Color::blue;
	attribute enumerationGreater : Boolean = Color::red > Color::blue;
	attribute enumerationLessEqual : Boolean = Color::red <= Color::blue;
	attribute enumerationGreaterEqual : Boolean = Color::red >= Color::blue;
	attribute enumerationAgainstNumber : Boolean = Level::low < Color::red;
	attribute partInstance : Boolean = widget < other;
	attribute partSequence : Boolean = widgets < 1;
	attribute metadataValue : Boolean = tagged.metadata < 1;
	attribute functionValue : Boolean = twice < 1;
	attribute nullValue : Boolean = null >= 1;
	attribute emptyValue : Boolean = nothing < 1;
}
