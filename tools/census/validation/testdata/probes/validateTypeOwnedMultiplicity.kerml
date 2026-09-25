// Census: validateTypeOwnedMultiplicity
// Expect: error: Only one multiplicity is allowed
package P {
  class C { multiplicity m1 [1]; multiplicity m2 [2]; }
}
