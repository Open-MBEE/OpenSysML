// Invalid: an invocation expression must invoke a behavior or behavioral feature (KerML 8.3.4.8 validateInvocationExpressionInstantiatedType; pilot validateInvocationExpressionInstantiatedType).
package K31InvokeClass {
    class C;
    feature f : C = C();
}
