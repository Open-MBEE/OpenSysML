// Invalid: every invocation argument must correspond to one input parameter of the invoked type (KerML 8.3.4.8 validateInvocationExpressionParameterRedefinition; pilot validateInvocationExpressionParameterRedefinition).
package K32TooManyArguments {
    class T;
    function F { in x : T; return r : T; }
    feature t : T;
    feature f : T = F(t, t);
}
