// Census: validateMetadataFeatureMetaclass
// Case: violating
// Ours: e[metadata-metaclass] Must have exactly one metaclass
// Pilot: e: Must have a concrete type
package P {
  class NotAMetaclass;
  class C { @NotAMetaclass; }
}
