// Census: validateTypeOwnedUnioningNotOne
// Case: violating
// Ours: e[type-unioning-not-one] Cannot have only one unioning
// Pilot: e: Cannot have only one unioning
package P {
  class A;
  class D unions A;
}
