// Census: validateFeatureMultiplicityDomain
// Expect: error: Multiplicity must have same featuring types as it feature
package P {
  class C {
    feature x {
      multiplicity m [1];
    }
  }
  class D;
  featuring of C::x::m by D;
}
