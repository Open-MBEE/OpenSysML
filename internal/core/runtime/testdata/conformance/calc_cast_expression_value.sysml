// An expression written as a value is kept by the evaluation type it is read as
// — a boolean body by BooleanEvaluation — and dropped by a type whose values it
// is not one of, so the cast selects a body still applicable afterwards
// (KerML 1.1 §8.3.4.9).
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	private import SequenceFunctions::*;

	calc def ExpressionCasts {
		out selectedByKeptBody : Integer[0..*] =
			select((1, 2, 3), { in v; v > 1 } as Performances::BooleanEvaluation);
		out keptAsEvaluation : Integer =
			size(({ in v; v > 1 }) as Performances::Evaluation);
		out droppedByScalarType : Integer =
			size(({ in v; v > 1 }) as Integer);
	}

	calc c : ExpressionCasts;
}
