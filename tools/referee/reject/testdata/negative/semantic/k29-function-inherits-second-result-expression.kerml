// Invalid: a function may have only one owned or inherited result expression (KerML 8.3.4.8 validateFunctionResultExpressionMembership; pilot validateFunctionResultExpressionMembership).
package K29FunctionInheritsSecondResultExpression {
    class T;
    feature a : T;
    feature b : T;
    function F { a }
    function G specializes F { b }
}
