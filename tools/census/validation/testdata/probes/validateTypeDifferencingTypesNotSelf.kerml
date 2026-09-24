// Census: validateTypeDifferencingTypesNotSelf
// Expect: error: Type cannot difference with itself
package P {
  class A;
  class D differences D, A;
}
