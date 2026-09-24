// Invalid: an expression may own at most one return parameter (KerML 8.3.4.8 validateExpressionResultParameterMembership; pilot validateExpressionResultParameterMembership).
package K28ExprTwoReturnParameters {
    class T;
    expr e {
        return a : T;
        return b : T;
    }
}
