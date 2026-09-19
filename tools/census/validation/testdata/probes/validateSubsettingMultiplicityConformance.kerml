// Census: validateSubsettingMultiplicityConformance
// Expect: warning: Subsetting/redefining feature should not have larger multiplicity upper bound
package P {
  class A { feature x [0..2]; }
  class B specializes A { feature y [0..5] subsets x; }
}
