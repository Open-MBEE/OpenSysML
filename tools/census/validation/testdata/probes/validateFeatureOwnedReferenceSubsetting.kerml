// Census: validateFeatureOwnedReferenceSubsetting
// Expect: error: At most one reference subsetting is allowed
package P {
  class C { feature a; feature b; feature c references a references b; }
}
