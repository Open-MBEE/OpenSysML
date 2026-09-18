// Invalid: an expression typed by a function that owns a result expression may not state a second (KerML 8.3.4.8 validateExpressionResultExpressionMembership; pilot validateExpressionResultExpressionMembership).
package K43ExprTypedByFunctionStatesSecondResultExpression {
    class T;
    feature a : T;
    feature b : T;
    function F { a }
    expr e : F { b }
}
