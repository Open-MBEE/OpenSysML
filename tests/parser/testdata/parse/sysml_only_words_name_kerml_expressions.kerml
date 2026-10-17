// KerML.xtext declares none of SysML's literals, so `action`, `part`,
// `attribute`, `calc` and the rest are ordinary names in a `.kerml` file
// wherever a name stands: before an operator, as a body's result, in a
// qualified name or a feature chain. Only a SysML declaration (`action a;`)
// keeps the word a keyword, for the notation pass to refuse.
package SysMLWordsAreKerMLNames {
	private import ScalarValues::*;

	struct Range {
		feature lowerBound : Real;
		feature upperBound : Real;
		feature action : Real;
		feature part : Real = action * 2;
		feature attribute : Boolean = action <= upperBound and part > lowerBound;
		feature calc : Real = if attribute ? action else part;
		feature item : Real[*] = (action, part);

		inv { action <= (upperBound - lowerBound) }
		inv { action < upperBound }
		expr nSteps : Integer { floor((upperBound - lowerBound) / action) }
		expr firstValue : Real { item#(1) }
		feature port : Real = Range::action + Range::part;
	}

	struct Sweep {
		feature range : Range;
		feature state : Real = range.action;
	}
}
