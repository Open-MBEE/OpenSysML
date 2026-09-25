// A built-in function binds named arguments to the parameters its library
// declaration names, in any order; a deferred operand named but not selected is
// never evaluated, and an optional parameter left unnamed is null.
package test {
	private import ScalarValues::*;

	calc def Named {
		out reorderedIf = ControlFunctions::'if'(elseValue = 1 / 0, thenValue = 1, test = true);
		out omittedElse : Integer[0..1] = ControlFunctions::'if'(thenValue = 1, test = false);
		out reorderedAnd = ControlFunctions::'and'(secondValue = 1 / 0 == 0, firstValue = false);
		out reorderedOr = ControlFunctions::'or'(secondValue = 2 > 1, firstValue = false);
		out reorderedImplies = ControlFunctions::'implies'(secondValue = 1 / 0 == 0, firstValue = false);
		out reorderedCoalesce = ControlFunctions::'??'(secondValue = 1 / 0, firstValue = 3);
		out reorderedSum0 = NumericalFunctions::sum0(zero = 0, collection = (1, 2, 3));
		out emptySum0 = NumericalFunctions::sum0(zero = 0, collection = ());
		out reorderedProduct1 = NumericalFunctions::product1(one = 1, collection = (2, 3, 4));
		out reorderedSelect : Integer[*] = ControlFunctions::select(selector = { in x; x > 1 }, collection = (1, 2, 3));
		out reorderedIndex = SequenceFunctions::'#'(index = 2, seq = (10, 20, 30));
		out namedSize = SequenceFunctions::size(seq = (1, 2, 3));
	}

	calc n : Named;
}
