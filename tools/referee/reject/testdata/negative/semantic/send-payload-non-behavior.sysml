// Invalid: a send payload that is an invocation must invoke a behavior or a behavioral feature (KerML 8.3.4.8 InvocationExpression validateInvocationExpressionInstantiatedType; pilot validateInvocationExpressionInstantiatedType).
package SendPayloadNonBehavior {
    attribute def Sig;
    part def Receiver;
    action def A {
        part target : Receiver;
        action s send Sig() to target;
    }
}
