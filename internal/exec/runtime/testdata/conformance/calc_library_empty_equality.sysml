// `null`, `()` and a collection filtered down to nothing all denote the empty
// sequence, so the equality and identity forms hold them equal to one another
// and unequal to any value.
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def EmptyEquality {
		out nullEqualsEmpty = null == ();
		out emptyEqualsNull = BaseFunctions::'=='((), null);
		out filteredEqualsNull = DataFunctions::'=='((1, 2)->select {in x; x > 2}, null);
		out emptyIdenticalToNull = BaseFunctions::'==='((), null);
		out emptyNotUnequalToNull = BaseFunctions::'!='((), null);
		out emptyNotZero = BaseFunctions::'=='((), 0);
		out emptyNotIdenticalToEmptyString = BaseFunctions::'!=='((), "");
	}

	calc c : EmptyEquality;
}
