// Every operator the library declares is also a function called by its quoted
// name, answering what the operator notation does.
package test {
	private import ScalarValues::*;

	calc def OperatorCalls {
		out sum = IntegerFunctions::'+'(1, 2);
		out negated = IntegerFunctions::'-'(5);
		out difference = RealFunctions::'-'(1.5, 2);
		out realSum = RealFunctions::'+'(1, 2);
		out realNegated = RealFunctions::'-'(5);
		out realProduct = RealFunctions::'*'(1099511627776, 1099511627776);
		out rationalSum = RationalFunctions::'+'(1, 2);
		out product = NumericalFunctions::'*'(3, 4);
		out quotient = IntegerFunctions::'/'(1, 4);
		out naturalQuotient = NaturalFunctions::'/'(6, 3);
		out remainder = IntegerFunctions::'%'(7, 3);
		out power = IntegerFunctions::'**'(2, 10);
		out caret = RealFunctions::'^'(2.0, -1);
		out concatenated = ScalarFunctions::'+'("a", "b");
		out equal = DataFunctions::'=='(1, 1);
		out equalAcrossKinds = BaseFunctions::'=='(1, 1.0);
		out realEqualsInteger = RealFunctions::'=='(2, 2.0);
		out naturalEquals = NaturalFunctions::'=='(2, 3);
		out unequal = BaseFunctions::'!='(1, 2);
		out identical = DataFunctions::'==='(1, 1);
		out notIdenticalAcrossKinds = BaseFunctions::'!=='(1, 1.0);
		out less = IntegerFunctions::'<'(1, 2);
		out atMost = RealFunctions::'<='(2.0, 2);
		out greater = ScalarFunctions::'>'("b", "a");
		out atLeast = NaturalFunctions::'>='(1, 2);
		out negation = BooleanFunctions::'not'(true);
		out exclusive = BooleanFunctions::'xor'(true, false);
		out either = BooleanFunctions::'|'(false, true);
		out both = BooleanFunctions::'&'(true, false);
		out dataNegation = DataFunctions::'not'(false);
		out range : Integer[0..*] = ScalarFunctions::'..'(1, 3);
		out emptyRange : Integer[0..*] = DataFunctions::'..'(3, 1);
		out indexed = BaseFunctions::'#'((10, 20, 30), 2);
		out joined : Integer[0..*] = BaseFunctions::','((1, 2), (3));
		out sameElements = CollectionFunctions::'=='((1, 2), (1, 2));
		out largest = DataFunctions::max(2, 7.5);
		out smallest = ScalarFunctions::min("pear", "apple");
	}

	calc c : OperatorCalls;
}
