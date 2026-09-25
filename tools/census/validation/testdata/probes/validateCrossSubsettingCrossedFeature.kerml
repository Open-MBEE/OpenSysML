// Census: validateCrossSubsettingCrossedFeature
// Expect: error: Cross subsetting must chain through an opposite end feature
package P {
  class A { feature ax : A; }
  class B { feature bx : A; }
  assoc L { end a : A crosses a.ax; end b : B; }
}
