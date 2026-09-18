// Census: validateMetadataUsageType
// Expect: error: A metadata usage must be typed by one metadata definition.
package P {
  part def NotMetadata;
  part c { metadata m : NotMetadata; }
}
