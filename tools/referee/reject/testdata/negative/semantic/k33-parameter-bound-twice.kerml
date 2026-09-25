// Invalid: an invocation cannot bind the same parameter twice (KerML 8.3.4.8 validateInvocationExpressionNoDuplicateParameterRedefinition; pilot validateInvocationExpressionNoDuplicateParameterRedefinition).
package K33ParameterBoundTwice {
    class T;
    function F { in x : T; return r : T; }
    feature t : T;
    feature f : T = F(x = t, x = t);
}
