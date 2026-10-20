// Census: validateTypeOwnedDifferencingNotOne
// Case: violating
// Ours: e[type-differencing-not-one] Cannot have only one differencing
// Pilot: e: Cannot have only one differencing
package P {
  class A;
  class D differences A;
}
