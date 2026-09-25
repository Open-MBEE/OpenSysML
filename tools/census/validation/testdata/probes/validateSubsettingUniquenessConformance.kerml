// Census: validateSubsettingUniquenessConformance
// Expect: error: Subsetting/redefining feature cannot be nonunique if subsetted/redefined feature is unique
package P {
  class A { feature x [*]; }
  class B specializes A { feature y [*] nonunique subsets x; }
}
