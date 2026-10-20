// Census: validateTypeOwnedIntersectingNotOne
// Case: violating
// Ours: e[type-intersecting-not-one] Cannot have only one intersecting
// Pilot: e: Cannot have only one intersecting
package P {
  class A;
  class D intersects A;
}
