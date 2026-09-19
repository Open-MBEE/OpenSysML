// Census: validateFeatureChainingFeatureConformance
// Expect: error: Must be a valid feature
package P {
  class B { feature y : B; }
  class A { alias z for B::y; }
  class K {
    feature k : A;
    feature r references k.z;
  }
}
