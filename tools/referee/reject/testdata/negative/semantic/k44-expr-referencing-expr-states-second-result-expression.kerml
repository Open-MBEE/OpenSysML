// Invalid: an expression reference-subsetting one that owns a result expression inherits it and may not state a second (KerML 8.3.4.8 validateExpressionResultExpressionMembership; pilot validateExpressionResultExpressionMembership).
package K44ExprReferencingExprStatesSecondResultExpression {
    class T;
    feature a : T;
    feature b : T;
    expr f { a }
    expr e ::> f { b }
}
