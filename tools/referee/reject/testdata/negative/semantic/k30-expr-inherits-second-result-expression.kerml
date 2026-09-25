// Invalid: an expression may have only one owned or inherited result expression (KerML 8.3.4.8 validateExpressionResultExpressionMembership; pilot validateExpressionResultExpressionMembership).
package K30ExprInheritsSecondResultExpression {
    class T;
    feature a : T;
    feature b : T;
    expr e { a }
    expr f subsets e { b }
}
