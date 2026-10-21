// Census: validateMetadataFeatureMetaclassNotAbstract
// Case: violating
// Ours: e[metadata-concrete-type] Must have a concrete type
// Pilot: e: Must have a concrete type
package P {
  abstract metaclass M;
  class C { @M; }
}
