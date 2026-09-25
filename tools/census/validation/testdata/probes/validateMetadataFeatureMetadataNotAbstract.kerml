// Census: validateMetadataFeatureMetadataNotAbstract
// Expect: error: Must have a concrete type
package P {
  abstract metaclass M;
  class C { @M; }
}
