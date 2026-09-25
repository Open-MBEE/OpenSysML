// Census: validateCrossSubsettingCrossingFeature
// Expect: error: Cross subsetting must be owned by one of two or more end features
package P {
  class A { feature p : A; feature q : A crosses p; }
}
