// Census: validateMetadataFeatureBody
// Expect: error: Must redefine an owning-type feature
package P {
  metaclass M { feature x : ScalarValues::Integer; }
  feature y : ScalarValues::Integer;
  class C { @M { z = 1; } }
}
