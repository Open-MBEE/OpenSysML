// Census: validateInvocationExpressionInstantiatedType
// Expect: error: Must invoke a behavior or a behavioral feature
package P {
  class A;
  feature f = A();
}
