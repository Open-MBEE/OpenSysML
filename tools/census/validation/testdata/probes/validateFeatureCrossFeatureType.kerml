// Census: validateFeatureCrossFeatureType
// Expect: error: Cross feature must have same type as feature
package P {
  class X;
  class A { feature ax : A; }
  class B { feature bx : X; }
  assoc L { end a : A crosses b.bx; end b : B crosses a.ax; }
}
