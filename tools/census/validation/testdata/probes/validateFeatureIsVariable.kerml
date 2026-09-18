// Census: validateFeatureIsVariable
// Expect: error: Must be owned by an occurrence type
package P {
  datatype D { var feature x : ScalarValues::Integer; }
}
