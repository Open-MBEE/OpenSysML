// Census: validateFeatureOwnedCrossSubsetting
// Expect: error: At most one cross subsetting is allowed
package P {
  class A { feature x : A; feature y : A; }
  assoc S { end a : A; end b : A crosses a.x crosses a.y; }
}
