// Census: validateFeatureReferenceExpressionReferentIsFeature
// Expect: error: Must be a valid feature
package P {
  class A;
  feature f : ScalarValues::Boolean = A;
}
