// SysML.xtext extends KerMLExpressions.xtext, not KerML.xtext, so `step`,
// `expr`, `inv`, `class`, `feature` and the other KerML-only literals are
// ordinary names in a `.sysml` file wherever a name stands: before an operator,
// as a body's result, in a qualified name or a feature chain (matched: pinned
// validator clean). Only a KerML declaration keeps the word a keyword.
package KerMLWordsAreSysMLNames {
	private import ScalarValues::*;

	attribute def Range {
		attribute lowerBound : Real;
		attribute upperBound : Real;
		attribute step : Real;
		attribute expr : Real = step * 2;
		attribute inv : Boolean = step <= upperBound and expr > lowerBound;
		attribute class : Real = if inv ? step else expr;
		attribute feature : Real[*] = (step, expr);

		assert constraint { step <= (upperBound - lowerBound) }
		assert constraint { step < upperBound }
		calc nSteps : Integer { floor((upperBound - lowerBound) / step) }
		calc firstValue : Real { feature#(1) }
		attribute function : Real = Range::step + Range::expr;
	}

	part def Sweep {
		part range : Range;
		attribute struct : Real = range.step;
		action run { assign struct := range.expr; }
	}
}
