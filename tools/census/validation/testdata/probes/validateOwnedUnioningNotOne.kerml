// Census: validateOwnedUnioningNotOne
// Expect: error: Cannot have only one unioning
package P {
  class A;
  class D unions A;
}
