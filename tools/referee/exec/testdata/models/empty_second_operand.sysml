package EmptyOperand {
    private import ScalarValues::*;
    private import NumericalFunctions::*;

    // KerML 1.1 §8.4.4.9.6: `x + y` is the invocation `'+'(x, y)`, whose `y` is
    // [0..1], so an empty second operand makes `+`/`-` the unary sign; `x` and
    // every operand of the other operators are [1], so an empty one is refused.
    calc def OpPlusNull { return : Integer = 1 + null; }
    calc def FnPlusNull { return : Integer = '+'(1, null); }
    calc def OpMinusNull { return : Integer = 3 - null; }
    calc def FnMinusNull { return : Integer = '-'(3, null); }
    calc def OpMinusEmpty { return : Integer = 3 - (); }
    calc def OpRealMinusNull { return : Real = 1.5 - null; }
    calc def OpNullPlus { return : Integer = null + 1; }
    calc def FnNullPlus { return : Integer = '+'(null, 1); }
    calc def OpTimesNull { return : Integer = 2 * null; }
    calc def OpLessNull { return : Boolean = 1 < null; }
}
