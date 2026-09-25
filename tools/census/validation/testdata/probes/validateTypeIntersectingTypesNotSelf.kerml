// Census: validateTypeIntersectingTypesNotSelf
// Expect: error: Type cannot intersect with itself
package P {
  class A;
  class D intersects D, A;
}
