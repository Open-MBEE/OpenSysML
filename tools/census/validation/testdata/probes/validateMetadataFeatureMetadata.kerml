// Census: validateMetadataFeatureMetadata
// Expect: error: Must have exactly one metaclass
package P {
  class NotAMetaclass;
  class C { @NotAMetaclass; }
}
