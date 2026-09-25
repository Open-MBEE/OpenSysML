// Census: validateTypeUnioningTypesNotSelf
// Expect: error: Type cannot union with itself
package P {
  class A;
  class D unions D, A;
}
